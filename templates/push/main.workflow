workflow "basic workflow" {
	on = "push"
	resolves = ["example"]
}

action "example" {
  uses = "./.github/action"
}