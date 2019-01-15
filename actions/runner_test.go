package actions

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/nektos/act/common"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestParseImageReference(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	tables := []struct {
		refIn  string
		refOut string
		ok     bool
	}{
		{"docker://myhost.com/foo/bar", "myhost.com/foo/bar", true},
		{"docker://ubuntu", "ubuntu", true},
		{"docker://ubuntu:18.04", "ubuntu:18.04", true},
		{"docker://cibuilds/hugo:0.53", "cibuilds/hugo:0.53", true},
		{"http://google.com:8080", "", false},
		{"./foo", "", false},
	}

	for _, table := range tables {
		refOut, ok := parseImageReference(table.refIn)
		assert.Equal(t, table.refOut, refOut)
		assert.Equal(t, table.ok, ok)
	}

}

func TestParseImageLocal(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	tables := []struct {
		pathIn     string
		contextDir string
		refTag     string
		ok         bool
	}{
		{"docker://myhost.com/foo/bar", "", "", false},
		{"http://google.com:8080", "", "", false},
		{"example/action1", "/example/action1", "action1:", true},
	}

	revision, _, err := common.FindGitRevision(".")
	assert.Nil(t, err)
	basedir, err := filepath.Abs("..")
	assert.Nil(t, err)
	for _, table := range tables {
		contextDir, refTag, ok := parseImageLocal(basedir, table.pathIn)
		assert.Equal(t, table.ok, ok, "ok match for %s", table.pathIn)
		if ok {
			assert.Equal(t, fmt.Sprintf("%s%s", basedir, table.contextDir), contextDir, "context dir doesn't match for %s", table.pathIn)
			assert.Equal(t, fmt.Sprintf("%s%s", table.refTag, revision), refTag)
		}
	}

}
func TestParseImageGithub(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	tables := []struct {
		image    string
		cloneURL string
		ref      string
		path     string
		ok       bool
	}{
		{"nektos/act", "https://github.com/nektos/act", "master", ".", true},
		{"nektos/act/foo", "https://github.com/nektos/act", "master", "foo", true},
		{"nektos/act@xxxxx", "https://github.com/nektos/act", "xxxxx", ".", true},
		{"nektos/act/bar/baz@zzzzz", "https://github.com/nektos/act", "zzzzz", "bar/baz", true},
		{"assimovt/actions-github-deploy/github-deploy@deployment-status-metadata", "https://github.com/assimovt/actions-github-deploy", "deployment-status-metadata", "github-deploy", true},
		{"nektos/zzzzundefinedzzzz", "", "", "", false},
	}

	for _, table := range tables {
		cloneURL, ref, path, ok := parseImageGithub(table.image)
		assert.Equal(t, table.ok, ok, "ok match for %s", table.image)
		if ok {
			assert.Equal(t, table.cloneURL, cloneURL.String())
			assert.Equal(t, table.ref, ref)
			assert.Equal(t, table.path, path)
		}
	}

}
