package main

import (
	"context"
	"log"
	"time"

	"github.com/rhyshort/github-to-omnifocus/internal"
	"github.com/rhyshort/github-to-omnifocus/internal/delta"
	"github.com/rhyshort/github-to-omnifocus/internal/gh"
	"github.com/rhyshort/github-to-omnifocus/internal/omnifocus"
)

// Version can be overridden at build time using PROJECT_VERSION in the makefile.
var Version = "development"

type OFCurrentState struct {
	Issues        []omnifocus.Task
	PRs           []omnifocus.Task
	Notifications []omnifocus.Task
	AuthoredPRs   []omnifocus.Task
}

type GHDesiredState struct {
	Issues        []gh.GitHubItem
	PRs           []gh.GitHubItem
	Notifications []gh.GitHubItem
	AuthoredPRs   []gh.GitHubItem
}

func main() {
	log.Printf("[main] Starting github2omnifocus; version: %s.", Version)

	c, err := internal.LoadConfig2()
	if err != nil {
		log.Fatal(err)
	}
	for _, v := range c {
		sync_github(v)
	}
}

func sync_github(c internal.GithubConfig) {

	// The due date we use is "end of today" which is 5pm local.
	dueDate := time.Now().Local()
	dueDate = time.Date(
		dueDate.Year(),
		dueDate.Month(),
		dueDate.Day(),
		17,
		0,
		0,
		0,
		dueDate.Location())

	// Gateways are used to access Omnifocus and GitHub
	og := omnifocus.Gateway{
		AppTag:                  c.AppTag,
		AssignedTag:             c.AssignedTag,
		AssignedProject:         c.AssignedProject,
		ReviewTag:               c.ReviewTag,
		ReviewProject:           c.ReviewProject,
		NotificationTag:         c.NotificationTag,
		NotificationsProject:    c.NotificationsProject,
		SetNotificationsDueDate: c.SetNotificationsDueDate,
		SetTaskmasterDueDate:    c.SetTaskmasterDueDate,
		TaskMasterTaskTag:       c.TaskMasterTaskTag,
		DueDate:                 dueDate,
		PendingChangesProject:   c.PendingChangesProject,
		PendingChangesTag:       c.PendingChangesTag,
	}
	ghg, err := gh.NewGitHubGateway(context.Background(), c.AccessToken, c.APIURL)
	if err != nil {
		log.Fatal(err)
	}

	// Retrieve our current (from Omnifocus) and desired (from GitHub) states
	currentState, err := GetOFState(og)
	if err != nil {
		log.Fatal(err)
	}
	desiredState, err := GetGitHubState(ghg)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Current state: %d issues; %d PRs; %d notifications.", len(currentState.Issues), len(currentState.PRs), len(currentState.Notifications))
	log.Printf("Desired state: %d issues; %d PRs; %d notifications.", len(desiredState.Issues), len(desiredState.PRs), len(desiredState.Notifications))

	// Create the delta and apply it to Omnifocus.
	// The awful looking *(d.Item.(*gh.GitHubItem)) casts are hacks, that we
	// know to be true because we know how Delta works. I suspect this is a
	// thing that generics will make easier as we can better smuggle the
	// types through Delta rather than using the interface.

	d := delta.Delta(toSet(desiredState.Issues), toSet(currentState.Issues))
	log.Printf("Found %d changes to apply to Issues", len(d))
	for _, d := range d {
		if d.Type == delta.Add {
			err := og.AddIssue(*(d.Item.(*gh.GitHubItem)))
			if err != nil {
				// should never fail
				log.Fatal(err)
			}
		} else if d.Type == delta.Remove {
			err := og.CompleteIssue(*(d.Item.(*omnifocus.Task)))
			if err != nil {
				// should never fail
				log.Fatal(err)
			}
		}
	}

	d = delta.Delta(toSet(desiredState.PRs), toSet(currentState.PRs))
	log.Printf("Found %d changes to apply to PRs", len(d))
	for _, d := range d {
		if d.Type == delta.Add {
			err := og.AddPR(*(d.Item.(*gh.GitHubItem)))
			if err != nil {
				// should never fail
				log.Fatal(err)
			}
		} else if d.Type == delta.Remove {
			err := og.CompletePR(*(d.Item.(*omnifocus.Task)))
			if err != nil {
				// should never fail
				log.Fatal(err)
			}
		}
	}

	d = delta.Delta(toSet(desiredState.AuthoredPRs), toSet(currentState.AuthoredPRs))
	log.Printf("Found %d changes to apply to PRs", len(d))
	for _, d := range d {
		if d.Type == delta.Add {
			err := og.AddAuthoredPR(*(d.Item.(*gh.GitHubItem)))
			if err != nil {
				// should never fail
				log.Fatal(err)
			}
		} else if d.Type == delta.Remove {
			err := og.CompletePR(*(d.Item.(*omnifocus.Task)))
			if err != nil {
				// should never fail
				log.Fatal(err)
			}
		}
	}

	d = delta.Delta(toSet(desiredState.Notifications), toSet(currentState.Notifications))
	log.Printf("Found %d changes to apply to Notifications", len(d))
	for _, d := range d {
		if d.Type == delta.Add {
			err := og.AddNotification(*(d.Item.(*gh.GitHubItem)))
			if err != nil {
				// should never fail
				log.Fatal(err)
			}
		} else if d.Type == delta.Remove {
			err := og.CompleteNotification(*(d.Item.(*omnifocus.Task)))
			if err != nil {
				// should never fail
				log.Fatal(err)
			}
		}
	}
}

func toSet[T delta.Keyed](l []T) map[string]T {
	// using the Key() as the map's hashkey allows for quicker lookup.
	// Without doing this, we are forced to essentially do the comparison as
	// a list comparison, looping over one list with an internal loop over the
	// other list, calling Key() all the time. For notifications in particular,
	// this can become large quickly: even a 50 item list ends up being in worst
	// case 2 * 50^2 = 5000 comparisons and Key() calls.
	// we build this here as it should be the same result as keying it on struct, and flipping
	// later
	r := map[string]T{}
	for index := range l {
		elem := l[index]
		r[elem.Key()] = elem
	}
	return r
}

// GetGitHubState retrieves the current state of our item types from GitHub
func GetGitHubState(ghg gh.GitHubGateway) (GHDesiredState, error) {
	ghState := GHDesiredState{}
	var err error

	ghState.Issues, err = ghg.GetIssues()
	if err != nil {
		return GHDesiredState{}, err
	}
	ghState.PRs, err = ghg.GetPRs()
	if err != nil {
		return GHDesiredState{}, err
	}

	ghState.AuthoredPRs, err = ghg.GetOpenPRs()
	if err != nil {
		return GHDesiredState{}, err
	}

	ghState.Notifications, err = ghg.GetNotifications()
	if err != nil {
		return GHDesiredState{}, err
	}

	return ghState, nil
}

// GetOFState retrieves the current state of our item types from Omnifocus
func GetOFState(og omnifocus.Gateway) (OFCurrentState, error) {
	ofState := OFCurrentState{}
	var err error

	ofState.Issues, err = og.GetIssues()
	if err != nil {
		return OFCurrentState{}, err
	}
	ofState.PRs, err = og.GetPRs()
	if err != nil {
		return OFCurrentState{}, err
	}
	ofState.Notifications, err = og.GetNotifications()
	if err != nil {
		return OFCurrentState{}, err
	}

	ofState.AuthoredPRs, err = og.GetAuthoredPRs()
	if err != nil {
		return OFCurrentState{}, err
	}

	return ofState, nil
}

// func exerciseGitHubClient(c internal.Config) error {

// 	ctx := context.Background()
// 	ts := oauth2.StaticTokenSource(
// 		&oauth2.Token{AccessToken: c.AccessToken},
// 	)
// 	tc := oauth2.NewClient(ctx, ts)

// 	// Passing APIURL as the uploadURL (2nd param) technically doesn't
// 	// work but we never upload so we're okay
// 	// list all repositories for the authenticated user
// 	client, err := github.NewEnterpriseClient(c.APIURL, c.APIURL, tc)
// 	if err != nil {
// 		return err
// 	}

// 	repos, _, err := client.Repositories.List(ctx, "", nil)
// 	if err != nil {
// 		return err
// 	}
// 	for _, repo := range repos {
// 		log.Printf("Repos: %s", *repo.Name)
// 	}

// 	return nil
// }

// // exerciseOmnifocus checks the OF scripts work
// func exerciseOmnifocus() error {
// 	tasks, err := omnifocus.OmnifocusTasksForQuery(omnifocus.TaskQuery{
// 		ProjectName: "GitHub Notifications",
// 		Tags:        []string{"github"},
// 	})
// 	if err != nil {
// 		return err
// 	}
// 	fmt.Printf("%v\n\n\n\n\n", tasks)

// 	err = omnifocus.EnsureTagExists(omnifocus.OmnifocusTag{Name: "github"})
// 	if err != nil {
// 		return err
// 	}

// 	task, err := omnifocus.AddNewOmnifocusTask(omnifocus.NewOmnifocusTask{
// 		ProjectName: "GitHub Reviews",
// 		Name:        "task title",
// 		Tags:        []string{"github"},
// 		Note:        "a note",
// 		DueDateMS:   100,
// 	})
// 	if err != nil {
// 		return err
// 	}
// 	fmt.Printf("%v\n\n\n\n\n", task)

// 	err = omnifocus.MarkOmnifocusTaskComplete(task)
// 	if err != nil {
// 		return err
// 	}

// 	return nil
// }
