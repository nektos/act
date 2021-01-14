![](https://github.com/nektos/act/wiki/img/logo-150.png)

# Overview [![push](https://github.com/nektos/act/workflows/push/badge.svg?branch=master&event=push)](https://github.com/nektos/act/actions) [![Join the chat at https://gitter.im/nektos/act](https://badges.gitter.im/nektos/act.svg)](https://gitter.im/nektos/act?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge) [![Go Report Card](https://goreportcard.com/badge/github.com/nektos/act)](https://goreportcard.com/report/github.com/nektos/act)

> "Think globally, <code>act</code> locally"

Run your [GitHub Actions](https://developer.github.com/actions/) locally! Why would you want to do this? Two reasons:

- **Fast Feedback** - Rather than having to commit/push every time you want to test out the changes you are making to your `.github/workflows/` files (or for any changes to embedded GitHub actions), you can use `act` to run the actions locally. The [environment variables](https://help.github.com/en/actions/configuring-and-managing-workflows/using-environment-variables#default-environment-variables) and [filesystem](https://help.github.com/en/actions/reference/virtual-environments-for-github-hosted-runners#filesystems-on-github-hosted-runners) are all configured to match what GitHub provides.
- **Local Task Runner** - I love [make](<https://en.wikipedia.org/wiki/Make_(software)>). However, I also hate repeating myself. With `act`, you can use the GitHub Actions defined in your `.github/workflows/` to replace your `Makefile`!

# How Does It Work?

When you run `act` it reads in your GitHub Actions from `.github/workflows/` and determines the set of actions that need to be run. It uses the Docker API to either pull or build the necessary images, as defined in your workflow files and finally determines the execution path based on the dependencies that were defined. Once it has the execution path, it then uses the Docker API to run containers for each action based on the images prepared earlier. The [environment variables](https://help.github.com/en/actions/configuring-and-managing-workflows/using-environment-variables#default-environment-variables) and [filesystem](https://help.github.com/en/actions/reference/virtual-environments-for-github-hosted-runners#filesystems-on-github-hosted-runners) are all configured to match what GitHub provides.

Let's see it in action with a [sample repo](https://github.com/cplee/github-actions-demo)!

![Demo](https://github.com/nektos/act/wiki/quickstart/act-quickstart-2.gif)

# Installation

To install with [Homebrew](https://brew.sh/), run:

`brew install act`

Alternatively, you can use the following:

`curl https://raw.githubusercontent.com/nektos/act/master/install.sh | sudo bash`

NOTE: Currently not compatible with Apple Silicon (ARM). This project relies on Docker which has not been ported to Apple Silicon yet.

If you are running Windows, download the [latest release](https://github.com/nektos/act/releases/latest) and add the binary into your PATH.  
If you are using [Chocolatey](https://chocolatey.org/) then run:  
`choco install act-cli`

If you are using [Scoop](https://scoop.sh/) then run:  
`scoop install act`

If you are running Arch Linux, you can install the [act](https://aur.archlinux.org/packages/act/) package with your favorite package manager:

`yay -S act`

If you are using NixOS or the Nix package manager on another platform you can install act globally by running

`nix-env -iA nixpkgs.act`

or in a shell by running

`nix-shell -p act`

# Commands

```
# Command structure:
act [event name to run] [flags]

# List the actions for the default event:
act -l

# List the actions for a specific event:
act workflow_dispatch -l

# Run the default (`push`) event:
act

# Run a specific event:
act pull_request

# Run a specific job:
act -j test

# Run in dry-run mode:
act -n

# Enable verbose-logging (can be used with any of the above commands)
act -v
```

# Flags

```
  -a, --actor string           user that triggered the event (default "nektos/act")
  -b, --bind                   bind working directory to container, rather than copy
      --defaultbranch string   the name of the main branch
  -C, --directory string       working directory (default ".")
  -n, --dryrun                 dryrun mode
      --env-file string        environment file to read and use as env in the containers (default ".env")
  -e, --eventpath string       path to event JSON file
  -g, --graph                  draw workflows
  -h, --help                   help for act
      --insecure-secrets       NOT RECOMMENDED! Doesn't hide secrets while printing logs.
  -j, --job string             run job
  -l, --list                   list workflows
  -P, --platform stringArray   custom image to use per platform (e.g. -P ubuntu-18.04=nektos/act-environments-ubuntu:18.04)
      --privileged             use privileged mode
  -p, --pull                   pull docker image(s) if already present
  -q, --quiet                  disable logging of output from steps
  -r, --reuse                  reuse action containers to maintain state
  -s, --secret stringArray     secret to make available to actions with optional value (e.g. -s mysecret=foo or -s mysecret)
      --secret-file string     file with list of secrets to read from (e.g. --secret-file .secrets)
  -v, --verbose                verbose output
  -w, --watch                  watch the contents of the local repo and run when files change
  -W, --workflows string       path to workflow file(s) (default "./.github/workflows/")
```

# Known Issues

MODULE_NOT_FOUND during `docker cp` command [#228](https://github.com/nektos/act/issues/228)

```
steps:
  - name: Checkout
    uses: actions/checkout@v2
    with:
      path: "your-action-root-directory"
```

# Runners

GitHub Actions offers managed [virtual environments](https://help.github.com/en/actions/reference/virtual-environments-for-github-hosted-runners) for running workflows. In order for `act` to run your workflows locally, it must run a container for the runner defined in your workflow file. Here are the images that `act` uses for each runner type:

| GitHub Runner  | Docker Image                                                      |
| -------------- | ----------------------------------------------------------------- |
| ubuntu-latest  | [node:12.6-buster-slim](https://hub.docker.com/_/buildpack-deps)  |
| ubuntu-20.04   | [node:12.6-buster-slim](https://hub.docker.com/_/buildpack-deps)  |
| ubuntu-18.04   | [node:12.6-buster-slim](https://hub.docker.com/_/buildpack-deps)  |
| ubuntu-16.04   | [node:12.6-stretch-slim](https://hub.docker.com/_/buildpack-deps) |
| windows-latest | `unsupported`                                                     |
| windows-2019   | `unsupported`                                                     |
| macos-latest   | `unsupported`                                                     |
| macos-10.15    | `unsupported`                                                     |

## Default runners are intentionally incomplete

These default images do **not** contain **all** the tools that GitHub Actions offers by default in their runners.

## Alternative runner images

If you need an environment that works just like the corresponding GitHub runner then consider using an image provided by [nektos/act-environments](https://github.com/nektos/act-environments):

- [nektos/act-environments-ubuntu:18.04](https://hub.docker.com/r/nektos/act-environments-ubuntu/tags) - built from the Packer file GitHub uses in [actions/virtual-environments](https://github.com/actions/runner).

:warning: :elephant: `*** WARNING - this image is >18GB 😱***`

## Use an alternative runner image

To use a different image for the runner, use the `-P` option:

```
act -P ubuntu-latest=nektos/act-environments-ubuntu:18.04
```

# Secrets

To run `act` with secrets, you can enter them interactively, supply them as environment variables or load them from a file. The following options are available for providing secrets:

- `act -s MY_SECRET=somevalue` - use `somevalue` as the value for `MY_SECRET`.
- `act -s MY_SECRET` - check for an environment variable named `MY_SECRET` and use it if it exists. If the environment variable is not defined, prompt the user for a value.
- `act --secret-file my.secrets` - load secrets values from `my.secrets` file.

# Configuration

You can provide default configuration flags to `act` by either creating a `./.actrc` or a `~/.actrc` file. Any flags in the files will be applied before any flags provided directly on the command line. For example, a file like below will always use the `nektos/act-environments-ubuntu:18.04` image for the `ubuntu-latest` runner:

```
# sample .actrc file
-P ubuntu-latest=nektos/act-environments-ubuntu:18.04
```

Additionally, act supports loading environment variables from an `.env` file. The default is to look in the working directory for the file but can be overridden by:

```
act --env-file my.env
```

# Skipping steps

Act adds a special environement variable `ACT` that can be used to skip a step that you
don't want to run locally. E.g. a step that posts a Slack message or bumps a version number.

```yml
- name: Some step
  if: ${{ !env.ACT }}
  run: |
    ...
```

# Events

Every [GitHub event](https://developer.github.com/v3/activity/events/types) is accompanied by a payload. You can provide these events in JSON format with the `--eventpath` to simulate specific GitHub events kicking off an action. For example:

```pull-request.json
{
  "pull_request": {
    "head": {
      "ref": "sample-head-ref"
    },
    "base": {
      "ref": "sample-base-ref"
    }
  }
}
```

```
act -e pull-request.json
```

Act will properly provide `github.head_ref` and `github.base_ref` to the action as expected.

# Support

Need help? Ask on [Gitter](https://gitter.im/nektos/act)!

# Contributing

Want to contribute to act? Awesome! Check out the [contributing guidelines](CONTRIBUTING.md) to get involved.

## Building from source

- Install Go tools 1.11.4+ - (https://golang.org/doc/install)
- Clone this repo `git clone git@github.com:nektos/act.git`
- Pull the default docker image `docker pull nektos/act-environments-ubuntu:18.04`
- Run unit tests with `make test`
- Build and install: `make install`
