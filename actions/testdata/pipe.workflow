workflow "test" {
  on = "push"
  resolves = ["test-action"]
}

action "test-action" {
  uses = "docker://alpine:3.9"
  runs = ["sh", "-c", "echo $IN | grep $OUT"]
  env = {
    IN = "foo"
    OUT = "foo"
  }
}