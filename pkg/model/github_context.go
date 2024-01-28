package model

import (
	"context"
	"fmt"
	"strings"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/common/git"
)

type GithubContext struct {
	Event            map[string]interface{} `json:"event"`
	EventPath        string                 `json:"event_path"`
	Workflow         string                 `json:"workflow"`
	RunID            string                 `json:"run_id"`
	RunNumber        string                 `json:"run_number"`
	Actor            string                 `json:"actor"`
	Repository       string                 `json:"repository"`
	EventName        string                 `json:"event_name"`
	Sha              string                 `json:"sha"`
	Ref              string                 `json:"ref"`
	RefName          string                 `json:"ref_name"`
	RefType          string                 `json:"ref_type"`
	HeadRef          string                 `json:"head_ref"`
	BaseRef          string                 `json:"base_ref"`
	Token            string                 `json:"token"`
	Workspace        string                 `json:"workspace"`
	Action           string                 `json:"action"`
	ActionPath       string                 `json:"action_path"`
	ActionRef        string                 `json:"action_ref"`
	ActionRepository string                 `json:"action_repository"`
	Job              string                 `json:"job"`
	JobName          string                 `json:"job_name"`
	RepositoryOwner  string                 `json:"repository_owner"`
	RetentionDays    string                 `json:"retention_days"`
	RunnerPerflog    string                 `json:"runner_perflog"`
	RunnerTrackingID string                 `json:"runner_tracking_id"`
	ServerURL        string                 `json:"server_url"`
	APIURL           string                 `json:"api_url"`
	GraphQLURL       string                 `json:"graphql_url"`
}

func asString(v interface{}) string {
	if v == nil {
		return ""
	} else if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func nestedMapLookup(m map[string]interface{}, ks ...string) (rval interface{}) {
	var ok bool

	if len(ks) == 0 { // degenerate input
		return nil
	}
	if rval, ok = m[ks[0]]; !ok {
		return nil
	} else if len(ks) == 1 { // we've reached the final key
		return rval
	} else if m, ok = rval.(map[string]interface{}); !ok {
		return nil
	} else { // 1+ more keys
		return nestedMapLookup(m, ks[1:]...)
	}
}

func withDefaultBranch(ctx context.Context, b string, event map[string]interface{}) map[string]interface{} {
	repoI, ok := event["repository"]
	if !ok {
		repoI = make(map[string]interface{})
	}

	repo, ok := repoI.(map[string]interface{})
	if !ok {
		common.Logger(ctx).Warnf("unable to set default branch to %v", b)
		return event
	}

	// if the branch is already there return with no changes
	if _, ok = repo["default_branch"]; ok {
		return event
	}

	repo["default_branch"] = b
	event["repository"] = repo

	return event
}

var findGitRef = git.FindGitRef
var findGitRevision = git.FindGitRevision

func (ghc *GithubContext) SetRef(ctx context.Context, defaultBranch string, repoPath string) {
	logger := common.Logger(ctx)

	// https://docs.github.com/en/actions/learn-github-actions/events-that-trigger-workflows
	// https://docs.github.com/en/developers/webhooks-and-events/webhooks/webhook-events-and-payloads
	switch ghc.EventName {
	case "pull_request_target":
		ghc.Ref = fmt.Sprintf("refs/heads/%s", ghc.BaseRef)
	case "pull_request", "pull_request_review", "pull_request_review_comment":
		ghc.Ref = fmt.Sprintf("refs/pull/%.0f/merge", ghc.Event["number"])
	case "deployment", "deployment_status":
		ghc.Ref = asString(nestedMapLookup(ghc.Event, "deployment", "ref"))
	case "release":
		ghc.Ref = fmt.Sprintf("refs/tags/%s", asString(nestedMapLookup(ghc.Event, "release", "tag_name")))
	case "push", "create", "workflow_dispatch":
		ghc.Ref = asString(ghc.Event["ref"])
	default:
		defaultBranch := asString(nestedMapLookup(ghc.Event, "repository", "default_branch"))
		if defaultBranch != "" {
			ghc.Ref = fmt.Sprintf("refs/heads/%s", defaultBranch)
		}
	}

	if ghc.Ref == "" {
		ref, err := findGitRef(ctx, repoPath)
		if err != nil {
			logger.Warningf("unable to get git ref: %v", err)
		} else {
			logger.Debugf("using github ref: %s", ref)
			ghc.Ref = ref
		}

		// set the branch in the event data
		if defaultBranch != "" {
			ghc.Event = withDefaultBranch(ctx, defaultBranch, ghc.Event)
		} else {
			ghc.Event = withDefaultBranch(ctx, "master", ghc.Event)
		}

		if ghc.Ref == "" {
			ghc.Ref = fmt.Sprintf("refs/heads/%s", asString(nestedMapLookup(ghc.Event, "repository", "default_branch")))
		}
	}
}

func (ghc *GithubContext) SetSha(ctx context.Context, repoPath string) {
	logger := common.Logger(ctx)

	// https://docs.github.com/en/actions/learn-github-actions/events-that-trigger-workflows
	// https://docs.github.com/en/developers/webhooks-and-events/webhooks/webhook-events-and-payloads
	switch ghc.EventName {
	case "pull_request_target":
		ghc.Sha = asString(nestedMapLookup(ghc.Event, "pull_request", "base", "sha"))
	case "deployment", "deployment_status":
		ghc.Sha = asString(nestedMapLookup(ghc.Event, "deployment", "sha"))
	case "push", "create", "workflow_dispatch":
		if deleted, ok := ghc.Event["deleted"].(bool); ok && !deleted {
			ghc.Sha = asString(ghc.Event["after"])
		}
	}

	if ghc.Sha == "" {
		_, sha, err := findGitRevision(ctx, repoPath)
		if err != nil {
			logger.Warningf("unable to get git revision: %v", err)
		} else {
			ghc.Sha = sha
		}
	}
}

func (ghc *GithubContext) SetRepositoryAndOwner(ctx context.Context, githubInstance string, remoteName string, repoPath string) {
	if ghc.Repository == "" {
		repo, err := git.FindGithubRepo(ctx, repoPath, githubInstance, remoteName)
		if err != nil {
			common.Logger(ctx).Warningf("unable to get git repo (githubInstance: %v; remoteName: %v, repoPath: %v): %v", githubInstance, remoteName, repoPath, err)
			return
		}
		ghc.Repository = repo
	}
	ghc.RepositoryOwner = strings.Split(ghc.Repository, "/")[0]
}

func (ghc *GithubContext) SetRefTypeAndName() {
	var refType, refName string

	// https://docs.github.com/en/actions/learn-github-actions/environment-variables
	if strings.HasPrefix(ghc.Ref, "refs/tags/") {
		refType = "tag"
		refName = ghc.Ref[len("refs/tags/"):]
	} else if strings.HasPrefix(ghc.Ref, "refs/heads/") {
		refType = "branch"
		refName = ghc.Ref[len("refs/heads/"):]
	} else if strings.HasPrefix(ghc.Ref, "refs/pull/") {
		refType = ""
		refName = ghc.Ref[len("refs/pull/"):]
	}

	if ghc.RefType == "" {
		ghc.RefType = refType
	}

	if ghc.RefName == "" {
		ghc.RefName = refName
	}
}

func (ghc *GithubContext) SetBaseAndHeadRef() {
	if ghc.EventName == "pull_request" || ghc.EventName == "pull_request_target" {
		if ghc.BaseRef == "" {
			ghc.BaseRef = asString(nestedMapLookup(ghc.Event, "pull_request", "base", "ref"))
		}

		if ghc.HeadRef == "" {
			ghc.HeadRef = asString(nestedMapLookup(ghc.Event, "pull_request", "head", "ref"))
		}
	}
}
