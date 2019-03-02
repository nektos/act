workflow "test" {
  on = "push"
  resolves = ["test-action"]
}

action "test-action" {
  uses = "./buildfail-action"
}