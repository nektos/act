# Security Policy

## Supported versions

Security fixes go into the latest release only. Upgrade to the newest
[release](https://github.com/nektos/act/releases) before you report a problem.

## Reporting a vulnerability

Do not open a public issue, discussion, or pull request for a security problem.

Report it privately through
[GitHub private vulnerability reporting](https://github.com/nektos/act/security/advisories/new).
Include:

- the act version (`act --version`) and your operating system
- the workflow, action, or command line that shows the problem
- what an attacker controls and what they gain

Maintainers will confirm the report, agree on a fix and a disclosure date with
you, and publish a GitHub security advisory when the fix is released.

## Threat model

act runs workflows on your machine with your Docker daemon and your
credentials. Only run workflows and actions that you trust. Self-hosted mode
(`-P <label>=-self-hosted`) runs steps directly on the host, without a container
boundary.

## Verifying downloads

New release archives and their `checksums.txt` carry signed build provenance.
Older releases do not. Verify a download with the GitHub CLI:

```sh
gh attestation verify act_Linux_x86_64.tar.gz --repo nektos/act \
  --signer-workflow nektos/act/.github/workflows/release.yml \
  --source-ref refs/tags/v0.2.92
```

Replace `v0.2.92` with the release you downloaded. `--signer-workflow` and
`--source-ref` reject provenance made by any other workflow or for another tag.
