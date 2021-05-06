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
	event := make(map[string]interface{})
	eventJSONBytes, err := ioutil.ReadFile(p)
	if err != nil {
		log.Errorf("%v", err)
		return ""
	}

	err = json.Unmarshal(eventJSONBytes, &event)
	if err != nil {
		log.Errorf("%v", err)
		return ""
	}

	keys := make([]string, 0, len(event))
	for k := range event {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	log.Debugf("json: %v", event)

	if event["action"] == "revoked" {
		return "github_app_authorization"
	}
	for _, k := range keys {
		switch k {
		case "check_run", "check_suite", "content_reference", "label", "marketplace_purchase", "milestone", "package",
			"project_card", "project_column", "project", "release", "security_advisory":
			return k
		case "alert":
			if event["ref"] != nil {
				return "code_scanning_alert"
			}
			switch event["action"] {
			case "created", "resolved", "reopened":
				return "secret_scanning_alert"
			}
			return "repository_vulnerability_alert"
		case "blocked_user":
			return "org_block"
		case "comment":
			if event["pull_request"] == nil {
				if event["issue"] != nil {
					return "issue_comment"
				}
				if event["discussion"] != nil {
					return "discussion_comment"
				}
				return "commit_comment"
			}
		case "client_payload":
			return "repository_dispatch"
		case "deployment":
			if event["deployment_status"] != nil {
				return "deployment_status"
			}
			return "deployment"
		case "member":
			if event["team"] != nil {
				return "membership"
			}
			return "member"
		case "membership":
			return "organization"
		case "ref":
			if event["ref_type"] != nil {
				if event["master_branch"] == nil {
					return "delete"
				}
				return "create"
			}
			if event["workflow"] != nil {
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
			if event["comment"] != nil {
				return "pull_request_review_comment"
			}
			if event["review"] != nil {
				return "pull_request_review"
			}
			return k
		case "pages":
			return "gollum"
		case "hook_id":
			if event["zen"] != nil {
				return "ping"
			}
			return "meta"
		case "issue":
			return "issues"
		case "installation":
			if event["repository_selection"] != nil {
				return "installation_repositories"
			}
			return "installation"
		case "build":
			return "page_build"
		case "repository":
			if event["starred_at"] != nil {
				return "star"
			}
			if event["team"] != nil {
				if event["action"] != nil {
					return "team"
				}
				return "team_add"
			}
			if event["state"] != nil {
				return "status"
			}
			if event["action"] != nil {
				if event["action"] == "started" {
					return "watch"
				}
				if event["action"] == "requested" || event["action"] == "completed" {
					return "workflow_run"
				}
				return "repository"
			}
			if event["status"] != nil {
				return "repository_import"
			}
			return "public"
		case "sponsorship":
			return "sponsorship"
		}
	}
	return ""
}
