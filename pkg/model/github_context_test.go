package model

import (
	"context"
	"fmt"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestSetRef(t *testing.T) {
	log.SetLevel(log.DebugLevel)

	oldFindGitRef := findGitRef
	oldFindGitRevision := findGitRevision
	defer func() { findGitRef = oldFindGitRef }()
	defer func() { findGitRevision = oldFindGitRevision }()

	findGitRef = func(ctx context.Context, file string) (string, error) {
		return "refs/heads/master", nil
	}

	findGitRevision = func(ctx context.Context, file string) (string, string, error) {
		return "", "1234fakesha", nil
	}

	tables := []struct {
		eventName string
		event     map[string]interface{}
		ref       string
	}{
		{
			eventName: "pull_request_target",
			event:     map[string]interface{}{},
			ref:       "refs/heads/master",
		},
		{
			eventName: "pull_request",
			event: map[string]interface{}{
				"number": 1234.,
			},
			ref: "refs/pull/1234/merge",
		},
		{
			eventName: "deployment",
			event: map[string]interface{}{
				"deployment": map[string]interface{}{
					"ref": "refs/heads/somebranch",
				},
			},
			ref: "refs/heads/somebranch",
		},
		{
			eventName: "release",
			event: map[string]interface{}{
				"release": map[string]interface{}{
					"tag_name": "v1.0.0",
				},
			},
			ref: "v1.0.0",
		},
		{
			eventName: "push",
			event: map[string]interface{}{
				"ref": "refs/heads/somebranch",
			},
			ref: "refs/heads/somebranch",
		},
		{
			eventName: "unknown",
			event: map[string]interface{}{
				"repository": map[string]interface{}{
					"default_branch": "main",
				},
			},
			ref: "refs/heads/main",
		},
		{
			eventName: "no-event",
			event:     map[string]interface{}{},
			ref:       "refs/heads/master",
		},
	}

	for _, table := range tables {
		t.Run(table.eventName, func(t *testing.T) {
			ghc := &GithubContext{
				EventName: table.eventName,
				BaseRef:   "master",
				Event:     table.event,
			}

			ghc.SetRef(context.Background(), "main", "/some/dir")

			assert.Equal(t, table.ref, ghc.Ref)
		})
	}

	t.Run("no-default-branch", func(t *testing.T) {
		findGitRef = func(ctx context.Context, file string) (string, error) {
			return "", fmt.Errorf("no default branch")
		}

		ghc := &GithubContext{
			EventName: "no-default-branch",
			Event:     map[string]interface{}{},
		}

		ghc.SetRef(context.Background(), "", "/some/dir")

		assert.Equal(t, "refs/heads/master", ghc.Ref)
	})
}

func TestSetSha(t *testing.T) {
	log.SetLevel(log.DebugLevel)

	oldFindGitRef := findGitRef
	oldFindGitRevision := findGitRevision
	defer func() { findGitRef = oldFindGitRef }()
	defer func() { findGitRevision = oldFindGitRevision }()

	findGitRef = func(ctx context.Context, file string) (string, error) {
		return "refs/heads/master", nil
	}

	findGitRevision = func(ctx context.Context, file string) (string, string, error) {
		return "", "1234fakesha", nil
	}

	tables := []struct {
		eventName string
		event     map[string]interface{}
		sha       string
	}{
		{
			eventName: "pull_request_target",
			event: map[string]interface{}{
				"pull_request": map[string]interface{}{
					"base": map[string]interface{}{
						"sha": "pr-base-sha",
					},
				},
			},
			sha: "pr-base-sha",
		},
		{
			eventName: "pull_request",
			event: map[string]interface{}{
				"number": 1234.,
			},
			sha: "1234fakesha",
		},
		{
			eventName: "deployment",
			event: map[string]interface{}{
				"deployment": map[string]interface{}{
					"sha": "deployment-sha",
				},
			},
			sha: "deployment-sha",
		},
		{
			eventName: "release",
			event:     map[string]interface{}{},
			sha:       "1234fakesha",
		},
		{
			eventName: "push",
			event: map[string]interface{}{
				"after":   "push-sha",
				"deleted": false,
			},
			sha: "push-sha",
		},
		{
			eventName: "unknown",
			event:     map[string]interface{}{},
			sha:       "1234fakesha",
		},
		{
			eventName: "no-event",
			event:     map[string]interface{}{},
			sha:       "1234fakesha",
		},
	}

	for _, table := range tables {
		t.Run(table.eventName, func(t *testing.T) {
			ghc := &GithubContext{
				EventName: table.eventName,
				BaseRef:   "master",
				Event:     table.event,
			}

			ghc.SetSha(context.Background(), "/some/dir")

			assert.Equal(t, table.sha, ghc.Sha)
		})
	}
}
