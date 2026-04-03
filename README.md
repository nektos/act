![act-logo](https://raw.githubusercontent.com/wiki/nektos/act/img/logo-150.png)

# Overview [![push](https://github.com/nektos/act/workflows/push/badge.svg?branch=master&event=push)](https://github.com/nektos/act/actions) [![Go Report Card](https://goreportcard.com/badge/github.com/nektos/act)](https://goreportcard.com/report/github.com/nektos/act) [![awesome-runners](https://img.shields.io/badge/listed%20on-awesome--runners-blue.svg)](https://github.com/jonico/awesome-runners)

> "Think globally, `act` locally"

Run your [GitHub Actions](https://developer.github.com/actions/) locally! Why would you want to do this? Two reasons:

- **Fast Feedback** - Rather than having to commit/push every time you want to test out the changes you are making to your `.github/workflows/` files (or for any changes to embedded GitHub actions), you can use `act` to run the actions locally. The [environment variables](https://help.github.com/en/actions/configuring-and-managing-workflows/using-environment-variables#default-environment-variables) and [filesystem](https://help.github.com/en/actions/reference/virtual-environments-for-github-hosted-runners#filesystems-on-github-hosted-runners) are all configured to match what GitHub provides.
- **Local Task Runner** - I love [make](<https://en.wikipedia.org/wiki/Make_(software)>). However, I also hate repeating myself. With `act`, you can use the GitHub Actions defined in your `.github/workflows/` to replace your `Makefile`!

> [!TIP]
> **Now Manage and Run Act Directly From VS Code!**<br/>
> Check out the [GitHub Local Actions](https://sanjulaganepola.github.io/github-local-actions-docs/) Visual Studio Code extension which allows you to leverage the power of `act` to run and test workflows locally without leaving your editor.

# How Does It Work?

When you run `act` it reads in your GitHub Actions from `.github/workflows/` and determines the set of actions that need to be run. It uses the Docker API to either pull or build the necessary images, as defined in your workflow files and finally determines the execution path based on the dependencies that were defined. Once it has the execution path, it then uses the Docker API to run containers for each action based on the images prepared earlier. The [environment variables](https://help.github.com/en/actions/configuring-and-managing-workflows/using-environment-variables#default-environment-variables) and [filesystem](https://docs.github.com/en/actions/using-github-hosted-runners/about-github-hosted-runners#file-systems) are all configured to match what GitHub provides.

Let's see it in action with a [sample repo](https://github.com/cplee/github-actions-demo)!

![Demo](https://raw.githubusercontent.com/wiki/nektos/act/quickstart/act-quickstart-2.gif)

# Installation

## macOS

### Prerequisites

`act` requires Docker to be installed and running. You have several options:

#### Option 1: Docker Desktop
Download and install [Docker Desktop for Mac](https://www.docker.com/products/docker-desktop/).

#### Option 2: Rancher Desktop
[Rancher Desktop](https://rancherdesktop.io/) is a popular alternative to Docker Desktop.

**Important:** When using Rancher Desktop, you need to ensure the Docker socket is properly configured:

```bash
# Check your docker context
docker context inspect --format '{{.Endpoints.docker.Host}}'

# Export the DOCKER_HOST environment variable
export DOCKER_HOST=$(docker context inspect --format '{{.Endpoints.docker.Host}}')

# For Rancher Desktop, you may also need to create a symlink
# if act can't find the Docker socket:
sudo ln -s "$HOME/.rd/docker.sock" /var/run/docker.sock
```

#### Option 3: Colima
[Colima](https://github.com/abiosoft/colima) is a lightweight container runtime for macOS.

```bash
# Install Colima
brew install colima docker

# Start Colima
colima start

# Export DOCKER_HOST
export DOCKER_HOST="unix://${HOME}/.colima/default/docker.sock"
```

### Install act

```bash
# Using Homebrew
brew install act

# Or using the install script
curl -fsSL https://raw.githubusercontent.com/nektos/act/master/install.sh | sudo bash
```

### macOS Troubleshooting

#### "Cannot connect to the Docker daemon" Error

If you see this error:
```
Error: Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?
```

Try these solutions:

1. **Check if Docker is running:**
   ```bash
   docker ps
   ```

2. **Set DOCKER_HOST for your specific setup:**
   ```bash
   # For Rancher Desktop
   export DOCKER_HOST=$(docker context inspect --format '{{.Endpoints.docker.Host}}')
   
   # For Colima
   export DOCKER_HOST="unix://${HOME}/.colima/default/docker.sock"
   ```

3. **Create a symlink (if needed):**
   ```bash
   # Find your docker.sock
   find ~ -name "docker.sock" 2>/dev/null
   
   # Create symlink (adjust path as needed)
   sudo ln -s "$HOME/.docker/run/docker.sock" /var/run/docker.sock
   ```

#### Apple Silicon (ARM64) Considerations

If you're using an M1/M2/M3 Mac, you may need to specify the container architecture:

```bash
# Run with linux/amd64 architecture
act --container-architecture linux/amd64
```

#### VS Code Integration

If using the [GitHub Local Actions](https://sanjulaganepola.github.io/github-local-actions-docs/) VS Code extension with Rancher Desktop:

1. Set the 'Act Command' setting to:
   ```
   act --container-architecture linux/amd64 -P ubuntu-latest=catthehacker/ubuntu:act-latest
   ```

2. Launch VS Code from terminal with DOCKER_HOST set:
   ```bash
   DOCKER_HOST=$(docker context inspect --format '{{.Endpoints.docker.Host}}') code
   ```

# Act User Guide

Please look at the [act user guide](https://nektosact.com) for more documentation.

# Support

Need help? Ask in [discussions](https://github.com/nektos/act/discussions)!

# Contributing

Want to contribute to act? Awesome! Check out the [contributing guidelines](CONTRIBUTING.md) to get involved.

## Manually building from source

- Install Go tools 1.20+ - (<https://golang.org/doc/install>)
- Clone this repo `git clone git@github.com:nektos/act.git`
- Run unit tests with `make test`
- Build and install: `make install`
