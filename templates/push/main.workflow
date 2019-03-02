workflow "basic workflow" {
	on = "push"
	resolves = ["example"]
}

action "example" {
  uses = "docker://.github/action"
}