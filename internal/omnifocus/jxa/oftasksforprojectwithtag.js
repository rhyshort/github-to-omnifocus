// Return the tasks for a project having a given tag
// Accepts a TaskQuery as JSON in an OSA_ARGS env var.
// Call it:
//   set -gx OSA_ARGS '{"projectName": "GitHub Notifications", "tags": ["github"]}'
//   osascript -l JavaScript oftasksforprojectwithtag.js | jq .
// Returns JSON array:
// [
//     {
//       "id": "iAKv1Uo8XqW",
//       "name": "cloudant/techspec-documents#257 Document modernize search project progress"
//     }, ...
// ]

/**
 * @typedef {Object} TaskQuery
 * @property {string} projectName
 * @property {string[]} tags
 */

function tasksForProjectWithTag(
    /** @type {TaskQuery} */ query
) {
    // @ts-ignore
    const ofApp = Application("OmniFocus")
    const ofDoc = ofApp.defaultDocument
    const project = ofDoc.flattenedProjects
        .whose({ name: query.projectName })[0];

    const tagFoundOrCreated = charTag => {
        const
            tags = ofDoc.flattenedTags.whose({
                name: charTag
            }),
            oTag = ofApp.Tag({
                name: charTag
            });
        return tags.length === 0 ? (
            (
                ofDoc.tags.push(oTag),
                oTag
            )
        ) : tags()[0]
    }

    const ofTags = query.tags.map((t) => {
        return tagFoundOrCreated(t)
    })

    // const ofAppTag = tagFoundOrCreated(appTag)
    // const ofTypeTag = tagFoundOrCreated(typeTag)

    // tasks is an array each task then has child tasks

    let tasks = project.tasks()
    project.tasks().forEach(task =>{
        tasks = tasks.concat(getChildTasks(task))
    })

    return tasks
        // removing this filter will give us the completed tasks, so we can then mark them as done in GH(E)
        .filter((task) => task.completed() === false)
        .filter((task) => {
            // Task must have all tags
            const tags = task.tags()
            for (var i = 0; i < ofTags.length; i++) {
                if (!tags.some(tag => tag.id() == ofTags[i].id())) {
                    return false
                }
            }
            return true
        })
        .map((task) => {
            return { "id": task.id(), "name": task.name(), "completed": task.completed(), "tags": task.tags().map(tag => tag.name()) };
        });
}

function getChildTasks(parent) {
    let all_tasks = parent.tasks()
    parent.tasks().forEach(t => {
        all_tasks = all_tasks.concat(getChildTasks(t))
    });
    return all_tasks;
}

ObjC.import('stdlib')
var args = JSON.parse($.getenv('OSA_ARGS'))
var out = tasksForProjectWithTag(args)
JSON.stringify(out)
