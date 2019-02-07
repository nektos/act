workflow "New workflow" {
  on = "push"
  resolves = ["branch-ref","commit-ref"]
}

action "branch-ref" {
  uses = "actions/docker/cli@master"
  args = "version"
}

action "commit-ref" {
  uses = "actions/docker/cli@c08a5fc9e0286844156fefff2c141072048141f6"
  args = "version"
}