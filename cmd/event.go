package cmd

import (
	"encoding/json"
	"io/ioutil"
	"sort"

	log "github.com/sirupsen/logrus"
)

// getEventFromFile returns proper event name based on content of event json file
// Based on version: // https://github.com/github/docs/blob/a926da8b08ce8230a1c0dd616f3d635cb175b055/content/developers/webhooks-and-events/webhook-events-and-payloads.md
// nolint:gocyclo
func getEventFromFile(p string) string {
	e := make(map[string]interface{})
	eventJSONBytes, err := ioutil.ReadFile(p)
	if err != nil {
		log.Errorf("%v", err)
		return ""
	}

	err = json.Unmarshal(eventJSONBytes, &e)
	if err != nil {
		log.Errorf("%v", err)
		return ""
	}

	keys := make([]string, 0, len(e))
	for k := range e {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	log.Debugf("json: %v", e)

	if e["action"] == "revoked" {
		return "github_app_authorization"
	}
	for _, k := range keys {
		switch k {
		case "check_run", "check_suite", "content_reference", "label", "marketplace_purchase", "milestone", "package",
			"project_card", "project_column", "project", "release", "security_advisory":
			return k
		case "alert":
			if e["ref"] != nil {
				return "code_scanning_alert"
			}
			switch e["action"] {
			case "created", "resolved", "reopened":
				return "secret_scanning_alert"
			}
			return "repository_vulnerability_alert"
		case "blocked_user":
			return "org_block"
		case "comment":
			if e["pull_request"] == nil {
				if e["issue"] != nil {
					return "issue_comment"
				}
				if e["discussion"] != nil {
					return "discussion_comment"
				}
				return "commit_comment"
			}
		case "client_payload":
			return "repository_dispatch"
		case "deployment":
			if e["deployment_status"] != nil {
				return "deployment_status"
			}
			return "deployment"
		case "member":
			if e["team"] != nil {
				return "membership"
			}
			return "member"
		case "membership":
			return "organization"
		case "ref":
			if e["ref_type"] != nil {
				if e["master_branch"] == nil {
					return "delete"
				}
				return "create"
			}
			if e["workflow"] != nil {
				return "workflow_dispatch"
			}
			return "push"
		case "key":
			return "deploy_key"
		case "discussion":
			return k
		case "forkee":
			return "fork"
		case "pull_request":
			if e["comment"] != nil {
				return "pull_request_review_comment"
			}
			if e["review"] != nil {
				return "pull_request_review"
			}
			return k
		case "pages":
			return "gollum"
		case "hook_id":
			if e["zen"] != nil {
				return "ping"
			}
			return "meta"
		case "issue":
			return "issues"
		case "installation":
			if e["repository_selection"] != nil {
				return "installation_repositories"
			}
			return "installation"
		case "build":
			return "page_build"
		case "repository":
			if e["starred_at"] != nil {
				return "star"
			}
			if e["team"] != nil {
				if e["action"] != nil {
					return "team"
				}
				return "team_add"
			}
			if e["state"] != nil {
				return "status"
			}
			if e["action"] != nil {
				if e["action"] == "started" {
					return "watch"
				}
				if e["action"] == "requested" || e["action"] == "completed" {
					return "workflow_run"
				}
				return "repository"
			}
			if e["status"] != nil {
				return "repository_import"
			}
			return "public"
		case "sponsorship":
			return "sponsorship"
		}
	}
	return ""
}
