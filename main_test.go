package main

import (
	"os"
	"runtime/debug"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMain(_ *testing.T) {
	os.Args = []string{"act", "--help"}
	main()
}

func TestResolveVersion(t *testing.T) {
	module := func(v string) *debug.BuildInfo {
		return &debug.BuildInfo{Main: debug.Module{Path: "github.com/nektos/act", Version: v}}
	}
	tests := []struct {
		name    string
		ldflags string
		info    *debug.BuildInfo
		ok      bool
		file    string
		want    string
	}{
		{"ldflags wins", "0.2.92", module("v0.2.93"), true, "0.2.91", "0.2.92"},
		{"go install of a tag", "", module("v0.2.92"), true, "0.2.91", "0.2.92"},
		{"go install of a prerelease tag", "", module("v0.3.0-rc.1"), true, "0.2.91", "0.3.0-rc.1"},
		{"go install of a branch", "", module("v0.2.93-0.20261028120000-abcdef123456"), true, "0.2.91", "0.2.93-0.20261028120000-abcdef123456"},
		{"dirty VCS build", "", module("v0.2.93-0.20261028120000-abcdef123456+dirty"), true, "0.2.91", "0.2.93-0.20261028120000-abcdef123456+dirty"},
		{"file is trimmed", "", nil, false, "0.2.91\n", "0.2.91"},
		{"devel build falls back to file", "", module("(devel)"), true, "0.2.91", "0.2.91"},
		{"no build info", "", nil, false, "0.2.91", "0.2.91"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, resolveVersion(tt.ldflags, tt.info, tt.ok, tt.file))
		})
	}
}
