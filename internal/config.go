package internal

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
)

type Config = map[string]GithubConfig

type GithubConfig struct {
	// API URL for GitHub
	APIURL string
	// Personal Access token
	AccessToken string
	// OF Tag applied to every task managed by the app (so we never mess with other tasks)
	AppTag string
	// OF Project that assigned issues are added to
	AssignedProject string
	// OF Tag for assigned items
	AssignedTag string
	// OF Project for PRs for review
	ReviewProject string
	// OF Tag for review items
	ReviewTag string
	// OF Project for notifications
	NotificationsProject string
	// OF Tag for notifications
	NotificationTag string
	// True if due date of today should be set on notifications
	SetNotificationsDueDate bool
	// True if app should attempt to set correct deadline for Task master apps
	SetTaskmasterDueDate bool
	// Tag used to id task master task
	TaskMasterTaskTag string
}

// LoadConfig loads JSON config from ~/.config/github2omnifocus/config.json
func LoadConfig2() (Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return make(Config), fmt.Errorf("could not find home dir: %v", err)
	}

	configFile := os.Getenv("G2O_CONFIG")
	if configFile == "" {
		configFile = "config.json"
	}
	configPath := path.Join(home, ".config", "github2omnifocus", configFile)

	var bytes []byte
	bytes, err = ioutil.ReadFile(configPath)
	if err != nil {
		return make(Config), fmt.Errorf("expected config.json at %s: %v", configPath, err)
	}

	c := make(Config)
	err = json.Unmarshal(bytes, &c)
	if err != nil {
		return c, fmt.Errorf("error unmarshalling config JSON from %s: %v", configPath, err)
	}

	log.Printf("Config loaded from %s:", configPath)

	for _, v := range c {
		log.Printf("  GitHub API server: %s", v.APIURL)
		if v.AccessToken != "" {
			log.Printf("  GitHub token: *****")
		} else {
			log.Printf("  GitHub token: <none, likely error!>")
		}
		log.Printf("  Omnifocus tag: %s", v.AppTag)
		log.Printf("  Omnifocus assigned issue project: %s", v.AssignedProject)
		log.Printf("  Omnifocus PR to review project: %s", v.ReviewProject)
		log.Printf("  Omnifocus notifications project: %s", v.NotificationsProject)
	}

	return c, nil
}
