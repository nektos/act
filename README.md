![](https://github.com/nektos/act/wiki/img/logo-150.png) 
# Overview [![Join the chat at https://gitter.im/nektos/act](https://badges.gitter.im/nektos/act.svg)](https://gitter.im/nektos/act?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge) [![Go Report Card](https://goreportcard.com/badge/github.com/nektos/act)](https://goreportcard.com/report/github.com/nektos/act) 

> "Think globally, <code>act</code> locally"

Run your [GitHub Actions](https://developer.github.com/actions/) locally! Why would you want to do this? Two reasons:

* **Fast Feedback** - Rather than having to commit/push every time you want test out the changes you are making to your `main.workflow` file (or for any changes to embedded GitHub actions), you can use `act` to run the actions locally. The [environment variables](https://developer.github.com/actions/creating-github-actions/accessing-the-runtime-environment/#environment-variables) and [filesystem](https://developer.github.com/actions/creating-github-actions/accessing-the-runtime-environment/#filesystem) are all configured to match what GitHub provides.
* **Local Task Runner** - I love [make](https://en.wikipedia.org/wiki/Make_(software)). However, I also hate repeating myself.  With `act`, you can use the GitHub Actions defined in your `main.workflow` file to replace your `Makefile`!  

# How Does It Work?
When you run `act` it reads in your GitHub Actions from `.github/main.workflow` and determines the set of actions that need to be run. It uses the Docker API to either pull or build the necessary images, as defined in your `main.workflow` file and finally determines the execution path based on the dependencies that were defined. Once it has the execution path, it then uses the Docker API to run containers for each action based on the images prepared earlier. The [environment variables](https://developer.github.com/actions/creating-github-actions/accessing-the-runtime-environment/#environment-variables) and [filesystem](https://developer.github.com/actions/creating-github-actions/accessing-the-runtime-environment/#filesystem) are all configured to match what GitHub provides.

Let's see it in action with a [sample repo](https://github.com/cplee/github-actions-demo)!

![Demo](https://github.com/nektos/act/wiki/quickstart/act-quickstart.gif)

# Installation
To install with [Homebrew](https://brew.sh/), run: 

```brew tap nektos/tap && brew install nektos/tap/act```

Alternatively, you can use the following: 

```curl  https://raw.githubusercontent.com/nektos/act/master/install.sh | sudo bash```

If you are running Arch Linux, you can install the [act](https://aur.archlinux.org/packages/act/) package with your favorite package manager:

```yay -S act```

# Commands

```
# List the actions
act -l

# Run the default (`push`) event:
act

# Run a specific event:
act pull_request

# Run a specific action:
act -a test

# Run in dry-run mode:
act -n

# Run in reuse mode to save state:
act -r

# Enable verbose-logging (can be used with any of the above commands)
act -v
```

# Secrets

To run `act` with secrets, you can enter them interactively or supply them as environment variables.
If you have a secret called `FOO` in your `main.workflow`, `act` will take whatever you have set as `FOO` in the session from which you are running `act`.
If `FOO` is unset, it will ask you interactively.

You can set environment variables for the current session by running `export FOO="zap"`, or globally in your `.profile`.
You can also set environment variables *per directory* using a tool such as [direnv](https://direnv.net/).
**Be careful not to expose secrets**:
You may want to `.gitignore` any files or folders containing secrets, and/or encrypt secrets.

# Skip Actions When Run in `act`

You may sometimes want to skip some actions when you're running a `main.workflow` in act, such as deployment.
You can achieve something similar by using a [filter](https://github.com/actions/bin/tree/master/filter) action, filtering on all [`GITHUB_ACTOR`](https://developer.github.com/actions/creating-github-actions/accessing-the-runtime-environment/#environment-variables)s *except* `nektos/act`, which is the `GITHUB_ACTOR` set by `act`.

```
action "Filter Not Act" {
  uses = "actions/bin/filter@3c0b4f0e63ea54ea5df2914b4fabf383368cd0da"
  args = "not actor nektos/act"
}
```

Just remember that GitHub actions will cancel all upcoming and concurrent actions on a neutral exit code.
To avoid prematurely cancelling actions, place this filter at the latest possible point in the build graph.

# Support

Need help? Ask on [Gitter](https://gitter.im/nektos/act)!

# Contributing

Want to contribute to act? Awesome! Check out the [contributing guidelines](CONTRIBUTING.md) to get involved.

## Building from source

* Install Go tools 1.11.4+ - (https://golang.org/doc/install)
* Clone this repo `git clone git@github.com:nektos/act.git`
* Run unit tests with `make check`
* Build and install: `make install`
