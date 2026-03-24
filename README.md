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

# Using Secrets with Act

When running GitHub Actions locally, you often need to provide secrets that your workflows use. Act supports several ways to pass secrets to your workflows:

## Command Line Secrets

Pass secrets directly via the `-s` or `--secret` flag:

```bash
# Single secret
act -s MY_SECRET=value

# Multiple secrets
act -s MY_SECRET=value -s ANOTHER_SECRET=another_value

# Secrets with special characters (use quotes)
act -s 'MY_SECRET=value with spaces'
```

## Secrets File

For workflows that require many secrets, you can use a secrets file. By default, Act looks for a `.secrets` file in your repository root:

```bash
# Create a .secrets file
cat > .secrets << EOF
MY_SECRET=my_value
API_KEY=sk-1234567890
DATABASE_URL=postgresql://localhost/db
EOF

# Run act (automatically loads .secrets)
act

# Or specify a custom secrets file
act --secret-file my-secrets.env
```

**Example `.secrets` file:**
```yaml
MY_SECRET=my_secret_value
GITHUB_TOKEN=ghp_xxxxxxxxxxxx
MULTILINE_SECRET: |
  line one
  line two
  line three
```

> [!IMPORTANT]
> Make sure to add `.secrets` to your `.gitignore` file to prevent accidentally committing sensitive data!

## Automatic GITHUB_TOKEN

Act automatically tries to use your GitHub CLI token if you have `gh` installed and authenticated:

```bash
# Check if gh is authenticated
gh auth status

# The GITHUB_TOKEN will be automatically available in your workflows
```

If you need to provide a specific token:

```bash
act -s GITHUB_TOKEN=ghp_your_token_here
```

## Example Workflow with Secrets

```yaml
name: Deploy
on: push
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Deploy to production
        env:
          API_KEY: ${{ secrets.API_KEY }}
          DATABASE_URL: ${{ secrets.DATABASE_URL }}
        run: |
          echo "Deploying with API key..."
          ./deploy-script.sh
```

Run it locally:
```bash
act -s API_KEY=your_api_key -s DATABASE_URL=your_db_url
```

## Security Best Practices

- **Never commit secrets** - Always add your secrets file to `.gitignore`
- **Use environment variables** for CI/CD, secrets files for local development
- **Rotate tokens regularly** - Especially if you suspect they may have been exposed
- **Use least privilege** - Only provide the secrets your workflow actually needs

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
