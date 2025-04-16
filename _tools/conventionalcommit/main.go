// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// This tool applies GitHub pull request labels based on the pull request's
// title, ensuring that it adheres to conventional commit standards used by this
// repository. It automatically removes any outdated labels that would have been
// assigned by this tool; and leaves any other labels untouched.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"maps"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/google/go-github/v69/github"
)

// conventionalLabels is a map of conventional commit types to their corresponding labels.
// See: https://gist.github.com/qoomon/5dfcdf8eec66a051ecd85625518cfd13
var conventionalLabels = map[string]string{
	"chore":    "conventional-commit/chore", // Miscellaneous commits e.g. modifying .gitignore
	"doc":      "conventional-commit/chore", // Alias for docs
	"docs":     "conventional-commit/chore", // Commits that affect documentation only
	"feat":     "conventional-commit/feat",  // Commits that add or remove a new feature to the API or UI
	"fix":      "conventional-commit/fix",   // Commits that fix an API or UI bug of a preceded feat commit
	"release":  "conventional-commit/chore", // Release-related commits
	"refactor": "conventional-commit/chore", // Commits that rewrite/restructure code without changing API/UI behavior
	"perf":     "conventional-commit/chore", // Special refactor commits that improve performance
	"style":    "conventional-commit/chore", // Commits that don't affect meaning (whitespace, formatting, etc)
	"test":     "conventional-commit/chore", // Commits that add missing tests or correct existing tests
	"build":    "conventional-commit/chore", // Commits affecting build components (tools, CI, dependencies, etc)
	"ops":      "conventional-commit/chore", // Commits affecting operational components (infrastructure, deployment, etc)
}

func main() {
	var (
		prOwner  string
		prRepo   string
		prNumber int = -1
	)

	// Get default values from environment if running under GHA.
	if os.Getenv("GITHUB_EVENT_NAME") == "pull_request" {
		if path := os.Getenv("GITHUB_EVENT_PATH"); path != "" {
			data, err := os.ReadFile(path)
			if err != nil {
				log.Fatalln(err)
			}
			var event struct {
				Number     int `json:"number"`
				Repository struct {
					Name  string `json:"name"`
					Owner struct {
						Login string `json:"login"`
					} `json:"owner"`
				} `json:"repository"`
			}
			if err := json.Unmarshal(data, &event); err != nil {
				log.Fatalln(err)
			}
			prRepo = event.Repository.Name
			prOwner = event.Repository.Owner.Login
			prNumber = event.Number
		}
	}

	flags := flag.NewFlagSet("conventionalcommit", flag.ExitOnError)
	flags.StringVar(&prOwner, "owner", prOwner, "The owner of the repository on which the pull request is made")
	flags.StringVar(&prRepo, "repo", prRepo, "The repository on which the pull request is made")
	flags.IntVar(&prNumber, "pr", prNumber, "The pull request number to apply labels to")

	if err := flags.Parse(os.Args[1:]); err != nil {
		log.Fatalln(err)
	}

	if prOwner == "" {
		log.Fatalln("Missing -owner flag")
	}
	if prRepo == "" {
		log.Fatalln("Missing -repo flag")
	}
	if prNumber <= 0 {
		log.Fatalln("Missing -pr flag")
	}

	ctx := context.Background()
	client := github.NewClient(nil)
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		client = client.WithAuthToken(token)
	}

	pr, _, err := client.PullRequests.Get(ctx, prOwner, prRepo, prNumber)
	if err != nil {
		log.Fatalln(err)
	}
	title := pr.GetTitle()
	log.Printf("The title of this pull request is: %q\n", title)

	prLabel, err := labelForTitle(title)
	if err != nil {
		log.Fatalln(err)
	}
	log.Printf("Conventional commit label for this pull request is: %q\n", prLabel)

	if _, _, err := client.Issues.AddLabelsToIssue(ctx, prOwner, prRepo, prNumber, []string{prLabel}); err != nil {
		log.Fatalf("Failed to add the label to the pull request: %v\n", err)
	}

	for _, label := range conventionalLabels {
		if label == prLabel {
			continue
		}
		if resp, err := client.Issues.RemoveLabelForIssue(ctx, prOwner, prRepo, prNumber, label); err != nil {
			if resp == nil || resp.StatusCode != http.StatusNotFound {
				log.Fatalf("Failed to remove label %q from pull request: %v\n", label, err)
			}
		} else {
			log.Printf("Removed outdated label from pull request: %q\n", label)
		}
	}
}

func labelForTitle(title string) (string, error) {
	regexText := fmt.Sprintf(`^(%s)(?:\(.+\))?: .*$`, strings.Join(slices.Collect(maps.Keys(conventionalLabels)), "|"))
	regex, err := regexp.Compile(regexText)
	if err != nil {
		return "", err
	}

	matches := regex.FindStringSubmatch(title)
	if matches == nil {
		return "", errors.New("title does not match expected conventional commit format")
	}

	return conventionalLabels[matches[1]], nil
}
