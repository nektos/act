![act-logo](https://github.com/nektos/act/wiki/img/logo-150.png)

# Overview [![push](https://github.com/nektos/act/workflows/push/badge.svg?branch=master&event=push)](https://github.com/nektos/act/actions) [![Join the chat at https://gitter.im/nektos/act](https://badges.gitter.im/nektos/act.svg)](https://gitter.im/nektos/act?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge) [![Go Report Card](https://goreportcard.com/badge/github.com/nektos/act)](https://goreportcard.com/report/github.com/nektos/act) [![awesome-runners](https://img.shields.io/badge/listed%20on-awesome--runners-blue.svg)](https://github.com/jonico/awesome-runners)

> "Think globally, `act` locally"

Run your [GitHub Actions](https://developer.github.com/actions/) locally! Why would you want to do this? Two reasons:

- **Fast Feedback** - Rather than having to commit/push every time you want to test out the changes you are making to your `.github/workflows/` files (or for any changes to embedded GitHub actions), you can use `act` to run the actions locally. The [environment variables](https://help.github.com/en/actions/configuring-and-managing-workflows/using-environment-variables#default-environment-variables) and [filesystem](https://help.github.com/en/actions/reference/virtual-environments-for-github-hosted-runners#filesystems-on-github-hosted-runners) are all configured to match what GitHub provides.
- **Local Task Runner** - I love [make](<https://en.wikipedia.org/wiki/Make_(software)>). However, I also hate repeating myself. With `act`, you can use the GitHub Actions defined in your `.github/workflows/` to replace your `Makefile`!

# How Does It Work?

When you run `act` it reads in your GitHub Actions from `.github/workflows/` and determines the set of actions that need to be run. It uses the Docker API to either pull or build the necessary images, as defined in your workflow files and finally determines the execution path based on the dependencies that were defined. Once it has the execution path, it then uses the Docker API to run containers for each action based on the images prepared earlier. The [environment variables](https://help.github.com/en/actions/configuring-and-managing-workflows/using-environment-variables#default-environment-variables) and [filesystem](https://docs.github.com/en/actions/using-github-hosted-runners/about-github-hosted-runners#file-systems) are all configured to match what GitHub provides.

Let's see it in action with a [sample repo](https://github.com/cplee/github-actions-demo)!

![Demo](https://github.com/nektos/act/wiki/quickstart/act-quickstart-2.gif)

# Act User Guide

Please look at the [act user guide](https://nektosact.com) for more documentation.

# Installation

## Necessary prerequisites for running `act`

`act` depends on `docker` to run workflows.

If you are using macOS, please be sure to follow the steps outlined in [Docker Docs for how to install Docker Desktop for Mac](https://docs.docker.com/docker-for-mac/install/).

If you are using Windows, please follow steps for [installing Docker Desktop on Windows](https://docs.docker.com/docker-for-windows/install/).

If you are using Linux, you will need to [install Docker Engine](https://docs.docker.com/engine/install/).

`act` is currently not supported with `podman` or other container backends (it might work, but it's not guaranteed). Please see [#303](https://github.com/nektos/act/issues/303) for updates.

## Installation through package managers

### [Homebrew](https://brew.sh/) (Linux/macOS)

[![homebrew version](https://img.shields.io/homebrew/v/act)](https://github.com/Homebrew/homebrew-core/blob/master/Formula/act.rb)

```shell
brew install act
```

or if you want to install version based on latest commit, you can run below (it requires compiler to be installed but Homebrew will suggest you how to install it, if you don't have it):

```shell
brew install act --HEAD
```

### [MacPorts](https://www.macports.org) (macOS)

[![MacPorts package](https://repology.org/badge/version-for-repo/macports/act-run-github-actions.svg)](https://repology.org/project/act-run-github-actions/versions)

```shell
sudo port install act
```

### [Chocolatey](https://chocolatey.org/) (Windows)

[![choco-shield](https://img.shields.io/chocolatey/v/act-cli)](https://community.chocolatey.org/packages/act-cli)

```shell
choco install act-cli
```

### [Scoop](https://scoop.sh/) (Windows)

[![scoop-shield](https://img.shields.io/scoop/v/act)](https://github.com/ScoopInstaller/Main/blob/master/bucket/act.json)

```shell
scoop install act
```

### [Winget](https://learn.microsoft.com/en-us/windows/package-manager/) (Windows)

[![Winget package](https://repology.org/badge/version-for-repo/winget/act-run-github-actions.svg)](https://repology.org/project/act-run-github-actions/versions)

```shell
winget install nektos.act
```

### [AUR](https://aur.archlinux.org/packages/act/) (Linux)

[![aur-shield](https://img.shields.io/aur/version/act)](https://aur.archlinux.org/packages/act/)

```shell
yay -Syu act
```

### [COPR](https://copr.fedorainfracloud.org/coprs/rubemlrm/act-cli/) (Linux)

```shell
dnf copr enable rubemlrm/act-cli
dnf install act-cli
```

### [Nix](https://nixos.org) (Linux/macOS)

[Nix recipe](https://github.com/NixOS/nixpkgs/blob/master/pkgs/development/tools/misc/act/default.nix)

Global install:

```sh
nix-env -iA nixpkgs.act
```

or through `nix-shell`:

```sh
nix-shell -p act
```

Using the latest [Nix command](https://nixos.wiki/wiki/Nix_command), you can run directly :

```sh
nix run nixpkgs#act
```

## Installation as GitHub CLI extension

Act can be installed as a [GitHub CLI](https://cli.github.com/) extension:

```sh
gh extension install https://github.com/nektos/gh-act
```

## Other install options

### Bash script

Run this command in your terminal:

```shell
curl -s https://raw.githubusercontent.com/nektos/act/master/install.sh | sudo bash
```

### Manual download

Download the [latest release](https://github.com/nektos/act/releases/latest) and add the path to your binary into your PATH.

# Example commands

```sh
# Command structure:
act [<event>] [options]
If no event name passed, will default to "on: push"
If actions handles only one event it will be used as default instead of "on: push"

# List all actions for all events:
act -l

# List the actions for a specific event:
act workflow_dispatch -l

# List the actions for a specific job:
act -j test -l

# Run the default (`push`) event:
act

# Run a specific event:
act pull_request

# Run a specific job:
act -j test

# Collect artifacts to the /tmp/artifacts folder:
act --artifact-server-path /tmp/artifacts

# Run a job in a specific workflow (useful if you have duplicate job names)
act -j lint -W .github/workflows/checks.yml

# Run in dry-run mode:
act -n

# Enable verbose-logging (can be used with any of the above commands)
act -v
```

## First `act` run

When running `act` for the first time, it will ask you to choose image to be used as default.
It will save that information to `~/.actrc`, please refer to [Configuration](#configuration) for more information about `.actrc` and to [Runners](#runners) for information about used/available Docker images.

## `GITHUB_TOKEN`

GitHub [automatically provides](https://docs.github.com/en/actions/security-guides/automatic-token-authentication#about-the-github_token-secret) a `GITHUB_TOKEN` secret when running workflows inside GitHub.

If your workflow depends on this token, you need to create a [personal access token](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/creating-a-personal-access-token) and pass it to `act` as a secret:

```bash
act -s GITHUB_TOKEN=[insert token or leave blank and omit equals for secure input]
```

If [GitHub CLI](https://cli.github.com/) is installed, the [`gh auth token`](https://cli.github.com/manual/gh_auth_token) command can be used to automatically pass the token to act

```bash
act -s GITHUB_TOKEN="$(gh auth token)"
```

**WARNING**: `GITHUB_TOKEN` will be logged in shell history if not inserted through secure input or (depending on your shell config) the command is prefixed with a whitespace.

# Known Issues

## Services

Services are not currently supported but are being worked on. See: [#173](https://github.com/nektos/act/issues/173)

## `MODULE_NOT_FOUND`

A `MODULE_NOT_FOUND` during `docker cp` command [#228](https://github.com/nektos/act/issues/228) can happen if you are relying on local changes that have not been pushed. This can get triggered if the action is using a path, like:

```yaml
- name: test action locally
  uses: ./
```

In this case, you _must_ use `actions/checkout@v2` with a path that _has the same name as your repository_. If your repository is called _my-action_, then your checkout step would look like:

```yaml
steps:
  - name: Checkout
    uses: actions/checkout@v2
    with:
      path: "my-action"
```

If the `path:` value doesn't match the name of the repository, a `MODULE_NOT_FOUND` will be thrown.

## `docker context` support

The current `docker context` isn't respected ([#583](https://github.com/nektos/act/issues/583)).

You can work around this by setting `DOCKER_HOST` before running `act`, with e.g:

```bash
export DOCKER_HOST=$(docker context inspect --format '{{.Endpoints.docker.Host}}')
```

# Runners

GitHub Actions offers managed [virtual environments](https://help.github.com/en/actions/reference/virtual-environments-for-github-hosted-runners) for running workflows. In order for `act` to run your workflows locally, it must run a container for the runner defined in your workflow file. Here are the images that `act` uses for each runner type and size:

| GitHub Runner   | Micro Docker Image               | Medium Docker Image                               | Large Docker Image                                 |
| --------------- | -------------------------------- | ------------------------------------------------- | -------------------------------------------------- |
| `ubuntu-latest` | [`node:16-buster-slim`][micro]   | [`catthehacker/ubuntu:act-latest`][docker_images] | [`catthehacker/ubuntu:full-latest`][docker_images] |
| `ubuntu-22.04`  | [`node:16-bullseye-slim`][micro] | [`catthehacker/ubuntu:act-22.04`][docker_images]  | `unavailable`                                      |
| `ubuntu-20.04`  | [`node:16-buster-slim`][micro]   | [`catthehacker/ubuntu:act-20.04`][docker_images]  | [`catthehacker/ubuntu:full-20.04`][docker_images]  |
| `ubuntu-18.04`  | [`node:16-buster-slim`][micro]   | [`catthehacker/ubuntu:act-18.04`][docker_images]  | [`catthehacker/ubuntu:full-18.04`][docker_images]  |

[micro]: https://hub.docker.com/_/buildpack-deps
[docker_images]: https://github.com/catthehacker/docker_images

Windows and macOS based platforms are currently **unsupported and won't work** (see issue [#97](https://github.com/nektos/act/issues/97))

## Please see [IMAGES.md](./IMAGES.md) for more information about the Docker images that can be used with `act`

## Default runners are intentionally incomplete

These default images do **not** contain **all** the tools that GitHub Actions offers by default in their runners.
Many things can work improperly or not at all while running those image.
Additionally, some software might still not work even if installed properly, since GitHub Actions are running in fully virtualized machines while `act` is using Docker containers (e.g. Docker does not support running `systemd`).
In case of any problems [please create issue](https://github.com/nektos/act/issues/new/choose) in respective repository (issues with `act` in this repository, issues with `nektos/act-environments-ubuntu:18.04` in [`nektos/act-environments`](https://github.com/nektos/act-environments) and issues with any image from user `catthehacker` in [`catthehacker/docker_images`](https://github.com/catthehacker/docker_images))

## Alternative runner images

If you need an environment that works just like the corresponding GitHub runner then consider using an image provided by [nektos/act-environments](https://github.com/nektos/act-environments):

- [`nektos/act-environments-ubuntu:18.04`](https://hub.docker.com/r/nektos/act-environments-ubuntu/tags) - built from the Packer file GitHub uses in [actions/virtual-environments](https://github.com/actions/runner).

:warning: :elephant: `*** WARNING - this image is >18GB 😱***`

- [`catthehacker/ubuntu:full-*`](https://github.com/catthehacker/docker_images/pkgs/container/ubuntu) - built from Packer template provided by GitHub, see [catthehacker/virtual-environments-fork](https://github.com/catthehacker/virtual-environments-fork) or [catthehacker/docker_images](https://github.com/catthehacker/docker_images) for more information

## Using local runner images

The `--pull` flag is set to true by default due to a breaking on older default docker images. This would pull the docker image everytime act is executed.

Set `--pull` to false if a local docker image is needed
```sh
  act --pull=false
```

## Use an alternative runner image

To use a different image for the runner, use the `-P` option.

```sh
act -P <platform>=<docker-image>
```

If your workflow uses `ubuntu-18.04`, consider below line as an example for changing Docker image used to run that workflow:

```sh
act -P ubuntu-18.04=nektos/act-environments-ubuntu:18.04
```

If you use multiple platforms in your workflow, you have to specify them to change which image is used.
For example, if your workflow uses `ubuntu-18.04`, `ubuntu-16.04` and `ubuntu-latest`, specify all platforms like below

```sh
act -P ubuntu-18.04=nektos/act-environments-ubuntu:18.04 -P ubuntu-latest=ubuntu:latest -P ubuntu-16.04=node:16-buster-slim
```

# Secrets

To run `act` with secrets, you can enter them interactively, supply them as environment variables or load them from a file. The following options are available for providing secrets:

- `act -s MY_SECRET=somevalue` - use `somevalue` as the value for `MY_SECRET`.
- `act -s MY_SECRET` - check for an environment variable named `MY_SECRET` and use it if it exists. If the environment variable is not defined, prompt the user for a value.
- `act --secret-file my.secrets` - load secrets values from `my.secrets` file.
  - secrets file format is the same as `.env` format

# Vars

To run `act` with repository variables that are acessible inside the workflow via ${{ vars.VARIABLE }}, you can enter them interactively or load them from a file. The following options are available for providing github repository variables:

- `act --var VARIABLE=somevalue` - use `somevalue` as the value for `VARIABLE`.
- `act --var-file my.variables` - load variables values from `my.variables` file.
  - variables file format is the same as `.env` format

# Configuration

You can provide default configuration flags to `act` by either creating a `./.actrc` or a `~/.actrc` file. Any flags in the files will be applied before any flags provided directly on the command line. For example, a file like below will always use the `nektos/act-environments-ubuntu:18.04` image for the `ubuntu-latest` runner:

```sh
# sample .actrc file
-P ubuntu-latest=nektos/act-environments-ubuntu:18.04
```

Additionally, act supports loading environment variables from an `.env` file. The default is to look in the working directory for the file but can be overridden by:

```sh
act --env-file my.env
```

`.env`:

```env
MY_ENV_VAR=MY_ENV_VAR_VALUE
MY_2ND_ENV_VAR="my 2nd env var value"
```

# Skipping jobs

You cannot use the `env` context in job level if conditions, but you can add a custom event property to the `github` context. You can use this method also on step level if conditions.

```yml
on: push
jobs:
  deploy:
    if: ${{ !github.event.act }} # skip during local actions testing
    runs-on: ubuntu-latest
    steps:
    - run: exit 0
```

And use this `event.json` file with act otherwise the Job will run:

```json
{
    "act": true
}
```

Run act like

```sh
act -e event.json
```

_Hint: you can add / append `-e event.json` as a line into `./.actrc`_

# Skipping steps

Act adds a special environment variable `ACT` that can be used to skip a step that you
don't want to run locally. E.g. a step that posts a Slack message or bumps a version number.
**You cannot use this method in job level if conditions, see [Skipping jobs](#skipping-jobs)**

```yml
- name: Some step
  if: ${{ !env.ACT }}
  run: |
    ...
```

# Events

Every [GitHub event](https://developer.github.com/v3/activity/events/types) is accompanied by a payload. You can provide these events in JSON format with the `--eventpath` to simulate specific GitHub events kicking off an action. For example:

```json
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

```sh
act pull_request -e pull-request.json
```

Act will properly provide `github.head_ref` and `github.base_ref` to the action as expected.

# Pass Inputs to Manually Triggered Workflows

Example workflow file

```yaml
on:
  workflow_dispatch:
    inputs:
      NAME:
        description: "A random input name for the workflow"
        type: string
      SOME_VALUE:
        description: "Some other input to pass"
        type: string

jobs:
  test:
    name: Test
    runs-on: ubuntu-latest

    steps:
      - name: Test with inputs
        run: |
          echo "Hello ${{ github.event.inputs.NAME }} and ${{ github.event.inputs.SOME_VALUE }}!"
```

## via input or input-file flag

- `act --input NAME=somevalue` - use `somevalue` as the value for `NAME` input.
- `act --input-file my.input` - load input values from `my.input` file.
  - input file format is the same as `.env` format

## via JSON

Example JSON payload file conveniently named `payload.json`

```json
{
  "inputs": {
    "NAME": "Manual Workflow",
    "SOME_VALUE": "ABC"
  }
}
```

Command for triggering the workflow

```sh
act workflow_dispatch -e payload.json
```

# GitHub Enterprise

Act supports using and authenticating against private GitHub Enterprise servers.
To use your custom GHE server, set the CLI flag `--github-instance` to your hostname (e.g. `github.company.com`).

Please note that if your GHE server requires authentication, we will use the secret provided via `GITHUB_TOKEN`.

Please also see the [official documentation for GitHub actions on GHE](https://docs.github.com/en/enterprise-server@3.0/admin/github-actions/about-using-actions-in-your-enterprise) for more information on how to use actions.

# Support

Need help? Ask on [Gitter](https://gitter.im/nektos/act)!

# Contributing

Want to contribute to act? Awesome! Check out the [contributing guidelines](CONTRIBUTING.md) to get involved.

## Manually building from source

- Install Go tools 1.20+ - (<https://golang.org/doc/install>)
- Clone this repo `git clone git@github.com:nektos/act.git`
- Run unit tests with `make test`
- Build and install: `make install`
