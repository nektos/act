package model

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
}
