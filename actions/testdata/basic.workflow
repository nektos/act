workflow "build-and-deploy" {
  on = "push"
  resolves = ["deploy"]
}

action "build" {
  uses = "./action1"
  args = "echo 'build'"
}

action "test" {
  uses = "docker://ubuntu:18.04"
  args = "env"
  needs = ["build"]
}

action "deploy" {
  uses = "./action2"
  runs = ["/bin/sh", "-c", "cat $GITHUB_EVENT_PATH"]
  needs = ["test"]
}