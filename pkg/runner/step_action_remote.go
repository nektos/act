package runner

import (
	"fmt"
	"regexp"
)

type remoteAction struct {
	URL  string
	Org  string
	Repo string
	Path string
	Ref  string
}

func (ra *remoteAction) CloneURL() string {
	return fmt.Sprintf("https://%s/%s/%s", ra.URL, ra.Org, ra.Repo)
}

func (ra *remoteAction) IsCheckout() bool {
	if ra.Org == "actions" && ra.Repo == "checkout" {
		return true
	}
	return false
}

func newRemoteAction(action string) *remoteAction {
	// GitHub's document[^] describes:
	// > We strongly recommend that you include the version of
	// > the action you are using by specifying a Git ref, SHA, or Docker tag number.
	// Actually, the workflow stops if there is the uses directive that hasn't @ref.
	// [^]: https://docs.github.com/en/actions/reference/workflow-syntax-for-github-actions
	r := regexp.MustCompile(`^([^/@]+)/([^/@]+)(/([^@]*))?(@(.*))?$`)
	matches := r.FindStringSubmatch(action)
	if len(matches) < 7 || matches[6] == "" {
		return nil
	}
	return &remoteAction{
		Org:  matches[1],
		Repo: matches[2],
		Path: matches[4],
		Ref:  matches[6],
		URL:  "github.com",
	}
}
