package omnifocus

import (
	"embed"
	"fmt"
	"iter"
	"log"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/rhyshort/github-to-omnifocus/internal/gh"
)

var (
	//go:embed jxa
	jxa embed.FS
)

// Task represents a task existing in Omnifocus
type Task struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Completed bool     `json:"completed"`
	Tags      []string `json:"tags"`
}

func (t Task) String() string {
	return fmt.Sprintf("OmnifocusTask: [%s] %s", t.Key(), t.Name)
}

// Key meets the Keyed interface used for creating delta operations in
// github2omnifocus.
func (t Task) Key() string {
	// The "key" here is the one used in github2omnifocus rather than the
	// Id used within Omnifocus. It's an opaque string we receive when creating
	// the task along with the task's actual title, though we can assume
	// it doesn't contain spaces. We stick it as the first thing in the task's
	// Name when we create the tasks.
	return strings.SplitN(t.Name, " ", 2)[0] //nolint:gomnd
}

func (t Task) GetTags() iter.Seq[string] {
	return slices.Values(t.Tags)
}

// TaskQuery defines a query to find Omnifocus tasks
type TaskQuery struct {
	ProjectName string   `json:"projectName"`
	Tags        []string `json:"tags"`
}

// NewOmnifocusTask defines a request to create a new task
type NewOmnifocusTask struct {
	ProjectName string   `json:"projectName"`
	Name        string   `json:"name"`
	Tags        []string `json:"tags"`
	Note        string   `json:"note"`
	DueDateMS   int64    `json:"dueDateMS"`
}

// Tag represents an Omnifocus tag
type Tag struct {
	Name string `json:"name"`
}

type Gateway struct {
	AppTag                  string
	AssignedTag             string
	AssignedProject         string
	ReviewTag               string
	ReviewProject           string
	NotificationTag         string
	NotificationsProject    string
	SetNotificationsDueDate bool
	SetTaskmasterDueDate    bool
	TaskMasterTaskTag       string
	DueDate                 time.Time
	PendingChangesProject   string
	PendingChangesTag       string
}

func (og *Gateway) GetIssues() ([]Task, error) {
	tasks, err := TasksForQuery(TaskQuery{
		ProjectName: og.AssignedProject,
		Tags:        []string{og.AppTag, og.AssignedTag},
	})
	if err != nil {
		return nil, err
	}
	return tasks, nil
}

func (og *Gateway) GetPRs() ([]Task, error) {
	tasks, err := TasksForQuery(TaskQuery{
		ProjectName: og.ReviewProject,
		Tags:        []string{og.AppTag, og.ReviewTag},
	})
	if err != nil {
		return nil, err
	}
	return tasks, nil
}

func (og *Gateway) GetAuthoredPRs() ([]Task, error) {
	tasks, err := TasksForQuery(TaskQuery{
		ProjectName: og.PendingChangesProject,
		Tags:        []string{og.AppTag, og.PendingChangesTag},
	})
	if err != nil {
		return nil, err
	}
	return tasks, nil
}

func (og *Gateway) GetNotifications() ([]Task, error) {
	tasks, err := TasksForQuery(TaskQuery{
		ProjectName: og.NotificationsProject,
		Tags:        []string{og.AppTag, og.NotificationTag},
	})
	if err != nil {
		return nil, err
	}
	return tasks, nil
}

func (og *Gateway) AddIssue(t gh.GitHubItem) error {
	log.Printf("AddIssue: %s", t)
	tags := []string{og.AppTag, og.AssignedTag, t.Repo}
	tags = append(tags, t.Labels...)
	if t.Milestone != "" {
		tags = append(tags, fmt.Sprintf("milestone: %s", t.Milestone))
	}

	task := NewOmnifocusTask{
		ProjectName: og.AssignedProject,
		Name:        t.Key() + " " + t.Title,
		Tags:        tags,
		Note:        t.HTMLURL,
	}

	if og.SetTaskmasterDueDate && og.isTaskMasterTask(task) {
		// attempt to set a TM due date
		deadline, err := og.deadline(tags)
		if err == nil {
			task.DueDateMS = deadline
		}
	}

	_, err := AddNewOmnifocusTask(task)
	if err != nil {
		return fmt.Errorf("error adding task: %v", err)
	}
	return nil
}

func (og *Gateway) isTaskMasterTask(task NewOmnifocusTask) bool {
	for _, tag := range task.Tags {
		if strings.EqualFold(tag, og.TaskMasterTaskTag) {
			return true
		}
	}
	return false
}

func (og *Gateway) deadline(tags []string) (int64, error) {
	// generic place in the year
	priorityOrderSuffix := []string{"H", "Q", "W"}

	// tm months
	arrowMonthAbbrv := []string{
		"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec",
	}

	isTimeRange := func(tag string) bool {
		for _, suffix := range priorityOrderSuffix {
			if strings.HasSuffix(tag, suffix) {
				return true
			}
		}
		return false
	}

	isMonth := func(tag string) (bool, time.Month) {
		for idx, month := range arrowMonthAbbrv {
			if strings.EqualFold(month, tag) {
				return true, time.Month(idx)
			}
		}
		return false, time.January
	}

	for _, tag := range tags {
		if isTimeRange(tag) {
			return getEndOfTimePeriod(tag)
		}

		if res, month := isMonth(tag); res {
			return getEndOfMonth(month), nil
		}
	}
	return -1, fmt.Errorf("No deadline present! tags: %v", tags)

}

func (og *Gateway) AddPR(t gh.GitHubItem) error {
	log.Printf("AddPR: %s", t)
	tags := []string{og.AppTag, og.ReviewTag}
	tags = append(tags, t.Labels...)
	tags = append(tags, t.Repo)
	_, err := AddNewOmnifocusTask(NewOmnifocusTask{
		ProjectName: og.ReviewProject,
		Name:        t.Key() + " " + t.Title,
		Tags:        tags,
		Note:        t.HTMLURL,
	})
	if err != nil {
		return fmt.Errorf("error adding task: %v", err)
	}
	return nil
}

func (og *Gateway) AddAuthoredPR(t gh.GitHubItem) error {
	log.Printf("AddAuhtoredPR: %s", t)
	tags := []string{og.AppTag, og.PendingChangesTag}
	tags = append(tags, t.Labels...)
	tags = append(tags, t.Repo)
	_, err := AddNewOmnifocusTask(NewOmnifocusTask{
		ProjectName: og.PendingChangesProject,
		Tags:        tags,
		Name:        t.Key() + " " + t.Title,
		Note:        t.HTMLURL,
	})
	return err
}

func (og *Gateway) AddNotification(t gh.GitHubItem) error {
	log.Printf("AddNotification: %s", t)
	newT := NewOmnifocusTask{
		ProjectName: og.NotificationsProject,
		Name:        t.Key() + " " + t.Title,
		Tags:        []string{og.AppTag, og.NotificationTag, t.Repo},
		Note:        t.HTMLURL,
	}
	if og.SetNotificationsDueDate {
		newT.DueDateMS = og.DueDate.UnixMilli()
	}
	_, err := AddNewOmnifocusTask(newT)
	if err != nil {
		return fmt.Errorf("error adding task: %v", err)
	}
	return nil
}

func (og *Gateway) CompleteIssue(t Task) error {
	log.Printf("CompleteIssue: %s", t)
	err := MarkOmnifocusTaskComplete(t)
	if err != nil {
		return fmt.Errorf("error completing task: %v", err)
	}
	return nil
}

func (og *Gateway) CompletePR(t Task) error {
	log.Printf("CompletePR: %s", t)
	err := MarkOmnifocusTaskComplete(t)
	if err != nil {
		return fmt.Errorf("error completing task: %v", err)
	}
	return nil
}

func (og *Gateway) CompleteNotification(t Task) error {
	log.Printf("CompleteNotification: %s", t)
	err := MarkOmnifocusTaskComplete(t)
	if err != nil {
		return fmt.Errorf("error completing task: %v", err)
	}
	return nil
}

func getEndOfTimePeriod(period string) (int64, error) {
	t := time.Now()

	suffix := period[len(period)-1:]
	num, err := strconv.Atoi(period[:len(period)-1])
	if err != nil {
		return -1, err
	}

	switch suffix {
	case "H":
		month := (6 * num) + 1
		return time.Date(t.Year(), time.Month(month), 1, -1, -1, -1, -1, time.Local).UnixMilli(), nil
	case "Q":
		month := (3 * num) + 1
		return time.Date(t.Year(), time.Month(month), 1, -1, -1, -1, -1, time.Local).UnixMilli(), nil
	case "W":
		day := (7 * num) + 1
		//TODO: adjust time for work days
		return time.Date(t.Year(), time.January, day, -1, -1, -1, -1, time.Local).UnixMilli(), nil

	}
	return -1, fmt.Errorf("Failed to calculate time increment")
}

func getEndOfMonth(month time.Month) int64 {
	t := time.Now()
	return time.Date(t.Year(), month+1, 1, -1, -1, -1, -1, time.Local).UnixMilli()
}
