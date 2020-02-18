![](https://github.com/nektos/act/wiki/img/logo-150.png) 
# Overview [![push](https://github.com/nektos/act/workflows/push/badge.svg?branch=master&event=push)](https://github.com/nektos/act/actions) [![Join the chat at https://gitter.im/nektos/act](https://badges.gitter.im/nektos/act.svg)](https://gitter.im/nektos/act?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge) [![Go Report Card](https://goreportcard.com/badge/github.com/nektos/act)](https://goreportcard.com/report/github.com/nektos/act) 


> "Think globally, <code>act</code> locally"

Run your [GitHub Actions](https://developer.github.com/actions/) locally! Why would you want to do this? Two reasons:

* **Fast Feedback** - Rather than having to commit/push every time you want test out the changes you are making to your `.github/workflows/` files (or for any changes to embedded GitHub actions), you can use `act` to run the actions locally. The [environment variables](https://help.github.com/en/actions/configuring-and-managing-workflows/using-environment-variables#default-environment-variables) and [filesystem](https://help.github.com/en/actions/reference/virtual-environments-for-github-hosted-runners#filesystems-on-github-hosted-runners) are all configured to match what GitHub provides.
* **Local Task Runner** - I love [make](https://en.wikipedia.org/wiki/Make_(software)). However, I also hate repeating myself.  With `act`, you can use the GitHub Actions defined in your `.github/workflows/` to replace your `Makefile`!  

# How Does It Work?
When you run `act` it reads in your GitHub Actions from `.github/workflows/` and determines the set of actions that need to be run. It uses the Docker API to either pull or build the necessary images, as defined in your workflow files and finally determines the execution path based on the dependencies that were defined. Once it has the execution path, it then uses the Docker API to run containers for each action based on the images prepared earlier. The [environment variables](https://help.github.com/en/actions/configuring-and-managing-workflows/using-environment-variables#default-environment-variables) and [filesystem](https://help.github.com/en/actions/reference/virtual-environments-for-github-hosted-runners#filesystems-on-github-hosted-runners) are all configured to match what GitHub provides.

Let's see it in action with a [sample repo](https://github.com/cplee/github-actions-demo)!

![Demo](https://github.com/nektos/act/wiki/quickstart/act-quickstart.gif)

# Installation
To install with [Homebrew](https://brew.sh/), run: 

```brew install nektos/tap/act```

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

# Run a specific job:
act -j test

# Run in dry-run mode:
act -n

# Run in reuse mode to save state:
act -r

# Enable verbose-logging (can be used with any of the above commands)
act -v
```

# Secrets

To run `act` with secrets, you can enter them interactively or supply them as environment variables. The following options are available for providing secrets:

* `act -s MY_SECRET=somevalue` - use `somevalue` as the value for `MY_SECRET`. 
* `act -s MY_SECRET` - check for an environment variable named `MY_SECRET` and use it if it exists.  If environment variable is not defined, prompt the user for a value.

# Support

Need help? Ask on [Gitter](https://gitter.im/nektos/act)!

# Contributing

Want to contribute to act? Awesome! Check out the [contributing guidelines](CONTRIBUTING.md) to get involved.

## Building from source

* Install Go tools 1.11.4+ - (https://golang.org/doc/install)
* Clone this repo `git clone git@github.com:nektos/act.git`
* Run unit tests with `make check`
* Build and install: `make install`
