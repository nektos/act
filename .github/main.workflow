workflow "check-and-release" {
  on = "push"
  resolves = ["release"]
}

action "check" {
  uses = "./.github/actions/check"
}

action "branch-filter" {
  needs = ["check"]
  uses = "actions/bin/filter@master"
  args = "tag v*"
}

# only release on `v*` tags
action "release" {
  needs = ["branch-filter"]
  uses = "docker://goreleaser/goreleaser:v0.97"
  args = "release"
  secrets = ["GITHUB_TOKEN"]
}

# local action for `make build`
action "build" {
  uses = "docker://goreleaser/goreleaser:v0.97"
  args = "--snapshot --rm-dist"
  secrets = ["SNAPSHOT_VERSION"]
}
