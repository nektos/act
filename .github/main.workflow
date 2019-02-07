workflow "check-and-release" {
  on = "push"
  resolves = ["release"]
}

action "check" {
  uses = "./.github/actions/check"
}

action "release-filter" {
  needs = ["check"]
  uses = "actions/bin/filter@master"
  args = "tag 'v*'"
}

# only release on `v*` tags
action "release" {
  needs = ["release-filter"]
  uses = "docker://goreleaser/goreleaser:v0.98"
  args = "release"
  secrets = ["GITHUB_TOKEN"]
}

# local action for `make build`
action "build" {
  uses = "docker://goreleaser/goreleaser:v0.98"
  args = "--snapshot --rm-dist"
  secrets = ["SNAPSHOT_VERSION"]
}

# local action for `make vendor`
action "vendor" {
  uses = "docker://golang:1.11.4"
  args = "go mod vendor"
}
