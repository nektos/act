workflow "detect-event" {
  on = "pull_request"
  resolves = ["build"]
}

action "build" {
  uses = "./action1"
  args = "echo 'build'"
}