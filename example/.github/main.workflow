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
  args = "echo 'test'"
  needs = ["build"]
}

action "deploy" {
  uses = "./action2"
  args = "echo 'deploy'"
  needs = ["test"]
}