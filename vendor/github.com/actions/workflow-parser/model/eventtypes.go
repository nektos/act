package model

import (
	"strings"
)

// IsAllowedEventType returns true if the event type is supported.
func IsAllowedEventType(eventType string) bool {
	_, ok := eventTypeWhitelist[strings.ToLower(eventType)]
	return ok
}

// IsMatchingEventType checks to see if the "flowOn" string from a flow's on attribute matches the incoming webhook of type eventType.
func IsMatchingEventType(flowOn, eventType string) bool {
	return strings.EqualFold(flowOn, eventType)
}

// https://developer.github.com/actions/creating-workflows/workflow-configuration-options/#events-supported-in-workflow-files
var eventTypeWhitelist = map[string]struct{}{
	"check_run":                   {},
	"check_suite":                 {},
	"commit_comment":              {},
	"create":                      {},
	"delete":                      {},
	"deployment":                  {},
	"deployment_status":           {},
	"fork":                        {},
	"gollum":                      {},
	"issue_comment":               {},
	"issues":                      {},
	"label":                       {},
	"member":                      {},
	"milestone":                   {},
	"page_build":                  {},
	"project_card":                {},
	"project_column":              {},
	"project":                     {},
	"public":                      {},
	"pull_request_review_comment": {},
	"pull_request_review":         {},
	"pull_request":                {},
	"push":                        {},
	"release":                     {},
	"repository_dispatch":         {},
	"status":                      {},
	"watch":                       {},
}

func AddAllowedEventType(s string) {
	eventTypeWhitelist[s] = struct{}{}
}

func RemoveAllowedEventType(s string) {
	delete(eventTypeWhitelist, s)
}
