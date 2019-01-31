workflow "New workflow" {
  on = "push"
  resolves = ["filter-version-before-deploy"]
}

action "filter-version-before-deploy" {
  uses = "actions/bin/filter@master"
  args = "tag z?[0-9]+\\.[0-9]+\\.[0-9]+"
}