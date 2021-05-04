# Contributing to Act

Help wanted!  We'd love your contributions to Act.  Please review the following guidelines before contributing.  Also, feel free to propose changes to these guidelines by updating this file and submitting a pull request.

* [I have a question...](#questions)
* [I found a bug...](#bugs)
* [I have a feature request...](#features)
* [I have a contribution to share...](#process)

## <a name="questions"></a> Have a Question?

Please don't open a GitHub issue for questions about how to use `act`, as the goal is to use issues for managing bugs and feature requests.  Issues that are related to general support will be closed and redirected to our gitter room.

For all support related questions, please ask the question in our gitter room: [nektos/act](https://gitter.im/nektos/act).

## <a name="bugs"></a> Found a Bug?

If you've identified a bug in `act`, please [submit an issue](#issue) to our GitHub repo: [nektos/act](https://github.com/nektos/act/issues/new).  Please also feel free to submit a [Pull Request](#pr) with a fix for the bug!

## <a name="features"></a> Have a Feature Request?

All feature requests should start with [submitting an issue](#issue) documenting the user story and acceptance criteria.  Again, feel free to submit a [Pull Request](#pr) with a proposed implementation of the feature.

## <a name="process"></a> Ready to Contribute

### <a name="issue"></a> Create an issue

Before submitting a new issue, please search the issues to make sure there isn't a similar issue doesn't already exist.

Assuming no existing issues exist, please ensure you include the following bits of information when submitting the issue to ensure we can quickly reproduce your issue:

* Version of `act`
* Platform (Linux, macOS, Windows)
* The complete `main.workflow` file used
* The complete command that was executed
* Any output from the command
* Details of the expected results and how they differed from the actual results

We may have additional questions and will communicate through the GitHub issue, so please respond back to our questions to help reproduce and resolve the issue as quickly as possible.

New issues can be created with in our [GitHub repo](https://github.com/nektos/act/issues/new).

### <a name="pr"></a>Pull Requests

Pull requests should target the `master` branch.  Please also reference the issue from the description of the pull request using [special keyword syntax](https://help.github.com/articles/closing-issues-via-commit-messages/) to auto close the issue when the PR is merged.  For example, include the phrase `fixes #14` in the PR description to have issue #14 auto close.

### <a name="style"></a> Styleguide

When submitting code, please make every effort to follow existing conventions and style in order to keep the code as readable as possible.  Here are a few points to keep in mind:

* Please run `go fmt ./...` before committing to ensure code aligns with go standards.
* All dependencies must be defined in the `go.mod` file.
* For details on the approved style, check out [Effective Go](https://golang.org/doc/effective_go.html).

Also, consider the original design principles:

* **Polyglot** - There will be no prescribed language or framework for developing the microservices.  The only requirement will be that the service will be run inside a container and exposed via an HTTP endpoint.
* **Cloud Provider** - At this point, the tool will assume AWS for the cloud provider and will not be written in a cloud agnostic manner.  However, this does not preclude refactoring to add support for other providers at a later time.
* **Declarative** - All resource administration will be handled in a declarative vs. imperative manner.  A file will be used to declared the desired state of the resources and the tool will simply assert the actual state matches the desired state.  The tool will accomplish this by generating CloudFormation templates.
* **Stateless** - The tool will not maintain its own state.  Rather, it will rely on the CloudFormation stacks to determine the state of the platform.
* **Secure** - All security will be managed by AWS IAM credentials.  No additional authentication or authorization mechanisms will be introduced.

### License

By contributing your code, you agree to license your contribution under the terms of the [MIT License](LICENSE.md).

All files are released with the MIT license.
