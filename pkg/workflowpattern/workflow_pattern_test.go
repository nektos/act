package workflowpattern

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatchPattern(t *testing.T) {
	kases := []struct {
		inputs       []string
		patterns     []string
		skipResult   bool
		filterResult bool
	}{
		{
			patterns:     []string{"*"},
			inputs:       []string{"path/with/slash"},
			skipResult:   true,
			filterResult: false,
		},
		{
			patterns:     []string{"path/a", "path/b", "path/c"},
			inputs:       []string{"meta", "path/b", "otherfile"},
			skipResult:   false,
			filterResult: false,
		},
		{
			patterns:     []string{"path/a", "path/b", "path/c"},
			inputs:       []string{"path/b"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"path/a", "path/b", "path/c"},
			inputs:       []string{"path/c", "path/b"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"path/a", "path/b", "path/c"},
			inputs:       []string{"path/c", "path/b", "path/a"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"path/a", "path/b", "path/c"},
			inputs:       []string{"path/c", "path/b", "path/d", "path/a"},
			skipResult:   false,
			filterResult: false,
		},
		{
			patterns:     []string{},
			inputs:       []string{},
			skipResult:   false,
			filterResult: false,
		},
		{
			patterns:     []string{"\\!file"},
			inputs:       []string{"!file"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"escape\\\\backslash"},
			inputs:       []string{"escape\\backslash"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{".yml"},
			inputs:       []string{"fyml"},
			skipResult:   true,
			filterResult: false,
		},
		// https://docs.github.com/en/actions/using-workflows/workflow-syntax-for-github-actions#patterns-to-match-branches-and-tags
		{
			patterns:     []string{"feature/*"},
			inputs:       []string{"feature/my-branch"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"feature/*"},
			inputs:       []string{"feature/your-branch"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"feature/**"},
			inputs:       []string{"feature/beta-a/my-branch"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"feature/**"},
			inputs:       []string{"feature/beta-a/my-branch"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"feature/**"},
			inputs:       []string{"feature/mona/the/octocat"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"main", "releases/mona-the-octocat"},
			inputs:       []string{"main"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"main", "releases/mona-the-octocat"},
			inputs:       []string{"releases/mona-the-octocat"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"*"},
			inputs:       []string{"main"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"*"},
			inputs:       []string{"releases"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"**"},
			inputs:       []string{"all/the/branches"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"**"},
			inputs:       []string{"every/tag"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"*feature"},
			inputs:       []string{"mona-feature"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"*feature"},
			inputs:       []string{"feature"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"*feature"},
			inputs:       []string{"ver-10-feature"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"v2*"},
			inputs:       []string{"v2"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"v2*"},
			inputs:       []string{"v2.0"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"v2*"},
			inputs:       []string{"v2.9"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"v[12].[0-9]+.[0-9]+"},
			inputs:       []string{"v1.10.1"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"v[12].[0-9]+.[0-9]+"},
			inputs:       []string{"v2.0.0"},
			skipResult:   false,
			filterResult: true,
		},
		// https://docs.github.com/en/actions/using-workflows/workflow-syntax-for-github-actions#patterns-to-match-file-paths
		{
			patterns:     []string{"*"},
			inputs:       []string{"README.md"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"*"},
			inputs:       []string{"server.rb"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"*.jsx?"},
			inputs:       []string{"page.js"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"*.jsx?"},
			inputs:       []string{"page.jsx"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"**"},
			inputs:       []string{"all/the/files.md"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"*.js"},
			inputs:       []string{"app.js"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"*.js"},
			inputs:       []string{"index.js"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"**.js"},
			inputs:       []string{"index.js"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"**.js"},
			inputs:       []string{"js/index.js"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"**.js"},
			inputs:       []string{"src/js/app.js"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"docs/*"},
			inputs:       []string{"docs/README.md"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"docs/*"},
			inputs:       []string{"docs/file.txt"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"docs/**"},
			inputs:       []string{"docs/README.md"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"docs/**"},
			inputs:       []string{"docs/mona/octocat.txt"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"docs/**/*.md"},
			inputs:       []string{"docs/README.md"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"docs/**/*.md"},
			inputs:       []string{"docs/mona/hello-world.md"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"docs/**/*.md"},
			inputs:       []string{"docs/a/markdown/file.md"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"**/docs/**"},
			inputs:       []string{"docs/hello.md"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"**/docs/**"},
			inputs:       []string{"dir/docs/my-file.txt"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"**/docs/**"},
			inputs:       []string{"space/docs/plan/space.doc"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"**/README.md"},
			inputs:       []string{"README.md"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"**/README.md"},
			inputs:       []string{"js/README.md"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"**/*src/**"},
			inputs:       []string{"a/src/app.js"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"**/*src/**"},
			inputs:       []string{"my-src/code/js/app.js"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"**/*-post.md"},
			inputs:       []string{"my-post.md"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"**/*-post.md"},
			inputs:       []string{"path/their-post.md"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"**/migrate-*.sql"},
			inputs:       []string{"migrate-10909.sql"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"**/migrate-*.sql"},
			inputs:       []string{"db/migrate-v1.0.sql"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"**/migrate-*.sql"},
			inputs:       []string{"db/sept/migrate-v1.sql"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"*.md", "!README.md"},
			inputs:       []string{"hello.md"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"*.md", "!README.md"},
			inputs:       []string{"README.md"},
			skipResult:   true,
			filterResult: true,
		},
		{
			patterns:     []string{"*.md", "!README.md"},
			inputs:       []string{"docs/hello.md"},
			skipResult:   true,
			filterResult: true,
		},
		{
			patterns:     []string{"*.md", "!README.md", "README*"},
			inputs:       []string{"hello.md"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"*.md", "!README.md", "README*"},
			inputs:       []string{"README.md"},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"*.md", "!README.md", "README*"},
			inputs:       []string{"README.doc"},
			skipResult:   false,
			filterResult: true,
		},
	}

	for _, kase := range kases {
		t.Run(strings.Join(kase.patterns, ","), func(t *testing.T) {
			patterns, err := CompilePatterns(kase.patterns...)
			assert.NoError(t, err)

			assert.EqualValues(t, kase.skipResult, Skip(patterns, kase.inputs, &StdOutTraceWriter{}), "skipResult")
			assert.EqualValues(t, kase.filterResult, Filter(patterns, kase.inputs, &StdOutTraceWriter{}), "filterResult")
		})
	}
}
