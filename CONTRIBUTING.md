# Contributing to Act

Help wanted! We'd love your contributions to Act. Please review the following guidelines before contributing. Also, feel free to propose changes to these guidelines by updating this file and submitting a pull request.

- [I have a question...](#questions)
- [I found a bug...](#bugs)
- [I have a feature request...](#features)
- [I have a contribution to share...](#process)

## <a id="questions"></a> Have a Question?

Please don't open a GitHub issue for questions about how to use `act`, as the goal is to use issues for managing bugs and feature requests. Issues that are related to general support will be closed and redirected to our gitter room.

For all support related questions, please ask the question in our gitter room: [nektos/act](https://gitter.im/nektos/act).

## <a id="bugs"></a> Found a Bug?

If you've identified a bug in `act`, please [submit an issue](#issue) to our GitHub repo: [nektos/act](https://github.com/nektos/act/issues/new). Please also feel free to submit a [Pull Request](#pr) with a fix for the bug!

## <a id="features"></a> Have a Feature Request?

All feature requests should start with [submitting an issue](#issue) documenting the user story and acceptance criteria. Again, feel free to submit a [Pull Request](#pr) with a proposed implementation of the feature.

## <a id="process"></a> Ready to Contribute

### <a id="issue"></a> Create an issue

Before submitting a new issue, please search the issues to make sure there isn't a similar issue doesn't already exist.

Assuming no existing issues exist, please ensure you include required information when submitting the issue to ensure we can quickly reproduce your issue.

We may have additional questions and will communicate through the GitHub issue, so please respond back to our questions to help reproduce and resolve the issue as quickly as possible.

New issues can be created with in our [GitHub repo](https://github.com/nektos/act/issues/new).

### <a id="pr"></a>Pull Requests

Pull requests should target the `master` branch. Please also reference the issue from the description of the pull request using [special keyword syntax](https://help.github.com/articles/closing-issues-via-commit-messages/) to auto close the issue when the PR is merged. For example, include the phrase `fixes #14` in the PR description to have issue #14 auto close.

### <a id="style"></a> Styleguide

When submitting code, please make every effort to follow existing conventions and style in order to keep the code as readable as possible. Here are a few points to keep in mind:

- Please run `go fmt ./...` before committing to ensure code aligns with go standards.
- We use [`golangci-lint`](https://golangci-lint.run/) for linting Go code, run `golangci-lint run --fix` before submitting PR. Editors such as Visual Studio Code or JetBrains IntelliJ; with Go support plugin will offer `golangci-lint` automatically.
- There are additional linters and formatters for files such as Markdown documents or YAML/JSON:
  - Please refer to the [Makefile](Makefile) or [`lint` job in our workflow](.github/workflows/checks.yml) to see how to those linters/formatters work.
  - You can lint codebase by running `go run main.go -j lint --env RUN_LOCAL=true` or `act -j lint --env RUN_LOCAL=true`
  - In `Makefile`, there are tools that require `npx` which is shipped with `nodejs`.
  - Our `Makefile` exports `GITHUB_TOKEN` from `~/.config/github/token`, you have been warned.
  - You can run `make pr` to cleanup dependencies, format/lint code and run tests.
- All dependencies must be defined in the `go.mod` file.
  - Advanced IDEs and code editors (like VSCode) will take care of that, but to be sure, run `go mod tidy` to validate dependencies.
- For details on the approved style, check out [Effective Go](https://golang.org/doc/effective_go.html).
- Before running tests, please be aware that they are multi-architecture so for them to not fail, you need to run `docker run --privileged --rm tonistiigi/binfmt --install amd64,arm64` before ([more info available in #765](https://github.com/nektos/act/issues/765)).

Also, consider the original design principles:

- **Polyglot** - There will be no prescribed language or framework for developing the microservices. The only requirement will be that the service will be run inside a container and exposed via an HTTP endpoint.
- **Cloud Provider** - At this point, the tool will assume AWS for the cloud provider and will not be written in a cloud agnostic manner. However, this does not preclude refactoring to add support for other providers at a later time.
- **Declarative** - All resource administration will be handled in a declarative vs. imperative manner. A file will be used to declared the desired state of the resources and the tool will simply assert the actual state matches the desired state. The tool will accomplish this by generating CloudFormation templates.
- **Stateless** - The tool will not maintain its own state. Rather, it will rely on the CloudFormation stacks to determine the state of the platform.
- **Secure** - All security will be managed by AWS IAM credentials. No additional authentication or authorization mechanisms will be introduced.

### License

By contributing your code, you agree to license your contribution under the terms of the [MIT License](LICENSE).

All files are released with the MIT license.
