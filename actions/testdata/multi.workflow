workflow "buildwf" {
  on = "push"
  resolves = ["build"]
}

action "build" {
  uses = "./action1"
  args = "echo 'build'"
}

workflow "deploywf" {
  on = "release"
  resolves = ["deploy"]
}

action "deploy" {
  uses = "./action2"
  runs = ["/bin/sh", "-c", "cat $GITHUB_EVENT_PATH"]
}
