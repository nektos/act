![act-logo](https://github.com/nektos/act/wiki/img/logo-150.png)

# Overview [![push](https://github.com/nektos/act/workflows/push/badge.svg?branch=master&event=push)](https://github.com/nektos/act/actions) [![Join the chat at https://gitter.im/nektos/act](https://badges.gitter.im/nektos/act.svg)](https://gitter.im/nektos/act?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge) [![Go Report Card](https://goreportcard.com/badge/github.com/nektos/act)](https://goreportcard.com/report/github.com/nektos/act) [![awesome-runners](https://img.shields.io/badge/listed%20on-awesome--runners-blue.svg)](https://github.com/jonico/awesome-runners)

> "Think globally, `act` locally"

Run your [GitHub Actions](https://developer.github.com/actions/) locally! Why would you want to do this? Two reasons:

- **Fast Feedback** - Rather than having to commit/push every time you want to test out the changes you are making to your `.github/workflows/` files (or for any changes to embedded GitHub actions), you can use `act` to run the actions locally. The [environment variables](https://help.github.com/en/actions/configuring-and-managing-workflows/using-environment-variables#default-environment-variables) and [filesystem](https://help.github.com/en/actions/reference/virtual-environments-for-github-hosted-runners#filesystems-on-github-hosted-runners) are all configured to match what GitHub provides.
- **Local Task Runner** - I love [make](<https://en.wikipedia.org/wiki/Make_(software)>). However, I also hate repeating myself. With `act`, you can use the GitHub Actions defined in your `.github/workflows/` to replace your `Makefile`!

# How Does It Work?

When you run `act` it reads in your GitHub Actions from `.github/workflows/` and determines the set of actions that need to be run. It uses the Docker API to either pull or build the necessary images, as defined in your workflow files and finally determines the execution path based on the dependencies that were defined. Once it has the execution path, it then uses the Docker API to run containers for each action based on the images prepared earlier. The [environment variables](https://help.github.com/en/actions/configuring-and-managing-workflows/using-environment-variables#default-environment-variables) and [filesystem](https://help.github.com/en/actions/reference/virtual-environments-for-github-hosted-runners#filesystems-on-github-hosted-runners) are all configured to match what GitHub provides.

Let's see it in action with a [sample repo](https://github.com/cplee/github-actions-demo)!

![Demo](https://github.com/nektos/act/wiki/quickstart/act-quickstart-2.gif)

# Installation

## Necessary prerequisites for running `act`

`act` depends on `docker` to run workflows.

If you are using macOS, please be sure to follow the steps outlined in [Docker Docs for how to install Docker Desktop for Mac](https://docs.docker.com/docker-for-mac/install/).

If you are using Windows, please follow steps for [installing Docker Desktop on Windows](https://docs.docker.com/docker-for-windows/install/).

If you are using Linux, you will need to [install Docker Engine](https://docs.docker.com/engine/install/).

`act` is currently not supported with `podman` or other container backends (it might work, but it's not guaranteed). Please see [#303](https://github.com/nektos/act/issues/303) for updates.

## Installation through package managers

### [Homebrew](https://brew.sh/) (Linux/macOS)

[![homebrew version](https://img.shields.io/homebrew/v/act)](https://github.com/nektos/homebrew-tap/blob/master/Formula/act.rb)

```shell
brew install act
```

### [MacPorts](https://www.macports.org) (macOS)

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

### [AUR](https://aur.archlinux.org/packages/act/) (Linux)

[![aur-shield](https://img.shields.io/aur/version/act)](https://aur.archlinux.org/packages/act/)

```shell
yay -S act
```

### Nix (Linux/macOS)

Global install:

```sh
nix-env -iA nixpkgs.act
```

or through `nix-shell`:

```sh
nix-shell -p act
```

### Go (Linux/Windows/macOS/any other platform supported by Go)

If you have Go 1.16+, you can install latest released version of `act` directly from source by running:

```sh
go install github.com/nektos/act@latest
```

or if you want to install latest unreleased version:

```sh
go install github.com/nektos/act@master
```

If you want a smaller binary size, run above commands with `-ldflags="-s -w"`

```sh
go install -ldflags="-s -w" github.com/nektos/act@...
```

## Other install options

### Bash script

Run this command in your terminal:

```shell
curl https://raw.githubusercontent.com/nektos/act/master/install.sh | sudo bash
```

### Manual download

Download the [latest release](https://github.com/nektos/act/releases/latest) and add the path to your binary into your PATH.

# Example commands

```sh
# Command structure:
act [<event>] [options]
If no event name passed, will default to "on: push"

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

## First `act` run

When running `act` for the first time, it will ask you to choose image to be used as default.
It will save that information to `~/.actrc`, please refer to [Configuration](#configuration) for more information about `.actrc` and to [Runners](#runners) for information about used/available Docker images.

# Flags

```none
  -a, --actor string                     user that triggered the event (default "nektos/act")
  -b, --bind                             bind working directory to container, rather than copy
      --container-architecture string    Architecture which should be used to run containers, e.g.: linux/amd64. If not specified, will use host default architecture. Requires Docker server API Version 1.41+. Ignored on earlier Docker server platforms.
      --container-daemon-socket string   Path to Docker daemon socket which will be mounted to containers (default "/var/run/docker.sock")
      --defaultbranch string             the name of the main branch
      --detect-event                     Use first event type from workflow as event that triggered the workflow
  -C, --directory string                 working directory (default ".")
  -n, --dryrun                           dryrun mode
      --env stringArray                  env to make available to actions with optional value (e.g. --env myenv=foo or --env myenv)
      --env-file string                  environment file to read and use as env in the containers (default ".env")
  -e, --eventpath string                 path to event JSON file
      --github-instance string           GitHub instance to use. Don't use this if you are not using GitHub Enterprise Server. (default "github.com")
  -g, --graph                            draw workflows
  -h, --help                             help for act
      --insecure-secrets                 NOT RECOMMENDED! Doesn't hide secrets while printing logs.
  -j, --job string                       run job
  -l, --list                             list workflows
      --no-recurse                       Flag to disable running workflows from subdirectories of specified path in '--workflows'/'-W' flag
  -P, --platform stringArray             custom image to use per platform (e.g. -P ubuntu-18.04=nektos/act-environments-ubuntu:18.04)
      --privileged                       use privileged mode
  -p, --pull                             pull docker image(s) even if already present
  -q, --quiet                            disable logging of output from steps
  -r, --reuse                            reuse action containers to maintain state
  -s, --secret stringArray               secret to make available to actions with optional value (e.g. -s mysecret=foo or -s mysecret)
      --secret-file string               file with list of secrets to read from (e.g. --secret-file .secrets) (default ".secrets")
      --use-gitignore                    Controls whether paths specified in .gitignore should be copied into container (default true)
      --userns string                    user namespace to use
  -v, --verbose                          verbose output
  -w, --watch                            watch the contents of the local repo and run when files change
  -W, --workflows string                 path to workflow file(s) (default "./.github/workflows/")
```

In case you want to pass a value for `${{ github.token }}`, you should pass `GITHUB_TOKEN` as secret: `act -s GITHUB_TOKEN=[insert token or leave blank for secure input]`.

# Known Issues

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

| GitHub Runner   | Micro Docker Image              | Medium Docker Image                                       | Large Docker Image                                         |
| --------------- | ------------------------------- | --------------------------------------------------------- | ---------------------------------------------------------- |
| `ubuntu-latest` | [`node:12-buster-slim`][micro]  | [`ghcr.io/catthehacker/ubuntu:act-latest`][docker_images] | [`ghcr.io/catthehacker/ubuntu:full-latest`][docker_images] |
| `ubuntu-20.04`  | [`node:12-buster-slim`][micro]  | [`ghcr.io/catthehacker/ubuntu:act-20.04`][docker_images]  | [`ghcr.io/catthehacker/ubuntu:full-20.04`][docker_images]  |
| `ubuntu-18.04`  | [`node:12-buster-slim`][micro]  | [`ghcr.io/catthehacker/ubuntu:act-18.04`][docker_images]  | [`ghcr.io/catthehacker/ubuntu:full-18.04`][docker_images]  |
| `ubuntu-16.04`  | [`node:12-stretch-slim`][micro] | [`ghcr.io/catthehacker/ubuntu:act-16.04`][docker_images]  | `unavailable`                                              |

[micro]: https://hub.docker.com/_/buildpack-deps
[docker_images]: https://github.com/catthehacker/docker_images

Below platforms are currently **unsupported and won't work** (see issue [#97](https://github.com/nektos/act/issues/97))

- `windows-latest`
- `windows-2019`
- `macos-latest`
- `macos-10.15`

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

- [`ghcr.io/catthehacker/ubuntu:full-*`](https://github.com/catthehacker/docker_images/pkgs/container/ubuntu) - built from Packer template provided by GitHub, see [catthehacker/virtual-environments-fork](https://github.com/catthehacker/virtual-environments-fork) or [catthehacker/docker_images](https://github.com/catthehacker/docker_images) for more information

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
act -P ubuntu-18.04=nektos/act-environments-ubuntu:18.04 -P ubuntu-latest=ubuntu:latest -P ubuntu-16.04=node:12-buster-slim
```

# Secrets

To run `act` with secrets, you can enter them interactively, supply them as environment variables or load them from a file. The following options are available for providing secrets:

- `act -s MY_SECRET=somevalue` - use `somevalue` as the value for `MY_SECRET`.
- `act -s MY_SECRET` - check for an environment variable named `MY_SECRET` and use it if it exists. If the environment variable is not defined, prompt the user for a value.
- `act --secret-file my.secrets` - load secrets values from `my.secrets` file.
  - secrets file format is the same as `.env` format

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

# Skipping steps

Act adds a special environment variable `ACT` that can be used to skip a step that you
don't want to run locally. E.g. a step that posts a Slack message or bumps a version number.

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
act -e pull-request.json
```

Act will properly provide `github.head_ref` and `github.base_ref` to the action as expected.

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

- Install Go tools 1.16+ - (<https://golang.org/doc/install>)
- Clone this repo `git clone git@github.com:nektos/act.git`
- Run unit tests with `make test`
- Build and install: `make install`
