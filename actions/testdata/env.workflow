workflow "test" {
  on = "push"
  resolves = [
    "test-action-repo",
    "test-action-ref",
  ]
}

action "test-action-repo" {
  uses = "docker://alpine:3.9"
  runs = ["sh", "-c", "echo $GITHUB_REPOSITORY | grep '^nektos/act$'"]
}

action "test-action-ref" {
  uses = "docker://alpine:3.9"
  runs = ["sh", "-c", "echo $GITHUB_REF | grep '^refs/'"]
}
