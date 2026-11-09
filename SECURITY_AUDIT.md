# act security audit (2026-10)

Scope: `main` at `7d8044a` (v0.2.91). Code fixes are on `security/vuln-fixes`. Workflow, release, and dependency hardening is on `security/supply-chain-hardening`. This document covers unpublished advisories and must stay on the private branch until they are published.

## Scans

| Tool | Result on main | Notes |
| --- | --- | --- |
| govulncheck | 0 called vulnerabilities, 8 module-level | x/net bump on the supply-chain branch clears all but x/crypto openpgp (GO-2026-5932, no fix released, not called) |
| gosec | 77 issues | Mostly G115 integer conversions in copied Docker CLI code and G304 file reads of operator paths. The real issues overlap the advisories below |
| staticcheck | 60 issues | Style only (ST1005 and similar) |
| gitleaks (full history) | 1405 commits, no leaks | |
| actionlint | Clean | Only the intentional failing fixtures `test-if.yml` and `test-expressions.yml` |
| zizmor (auditor) | 5 high, 4 medium | Supply-chain branch: 0 high, 2 medium (Codecov secret outside an environment, promote persisted credentials; both need owner settings) |
| Dockerfile pins | 4 unpinned base images | Pinned by digest on the supply-chain branch; `pull.Dockerfile` uses a build `ARG IMAGE` and cannot be pinned |
| go.mod review | x/net and siblings behind | Bumped on the supply-chain branch. go-git already at 5.19.2 |

## Findings

Severity is the reporter's unless noted. Status is relative to `main` before this audit.

| Id | Severity | Area | Status | Fix | Breaking |
| --- | --- | --- | --- | --- | --- |
| GHSA-7x4h-3p32-rrc3 | Critical | `container.options` overrides CapDrop, AppArmor, seccomp, masked paths | Exploitable | 94e9ff0: operator CapDrop always survives; security-opt, privileged, cap-add rejected under policy | Yes, under policy |
| GHSA-v5hg-4ch3-5f5f | Critical | `--env-file`, `--label-file`, `seccomp=<file>` read host files | Exploitable | 94e9ff0: file-reading options rejected before parse under policy | Yes, under policy |
| GHSA-hf9j-r4wr-4p36 | Critical | Host bind mounts through options and `volumes:` | Exploitable | 94e9ff0, 3b73e52: host binds and absolute workflow volumes rejected under policy | Yes, under policy |
| GHSA-3m58-6wgp-2p53 | Critical | Host PID/IPC/UTS/userns, devices, GPUs | Exploitable | 94e9ff0 | Yes, under policy |
| GHSA-45wq-m944-9xrx | Critical | Same merge path for service containers | Exploitable | 94e9ff0: services always get the policy | Yes, under policy |
| GHSA-9j7q-g3ch-2283 | High | `uses:` and `runs.main/pre/post` path traversal | Exploitable | b6a56a0 (plus 439f1b1 for host mode) | Yes, for actions that point outside their repo |
| GHSA-x66w-c5wj-w4pc | High | Host-mode extraction writes outside destination | Exploitable | 439f1b1: `os.Root` plus `filepath.IsLocal` in `CopyCollector` and `HostEnvironment.Copy` | No |
| GHSA-jg8p-h7rv-ccmc | High | `NODE_OPTIONS` through `GITHUB_ENV` | Exploitable | 64674e6: refused, matching the GitHub runner | Minor |
| GHSA-88g4-mq6c-w2c9 | High | `./.actrc` overrides operator hardening | Exploitable | 2be4360: project `.actrc` limited to an allowlist unless `--trust-project-actrc` | Yes, for project files using other flags |
| GHSA-f4c3-f4xq-56w5 | High | Artifact server LAN binding and forgeable v4 signatures | Mostly fixed in b9d3535, 2bfe1c1 (v0.2.90) | aba07a9 covers residuals | No |
| GHSA-733g-98g7-3hqv | High | Missing auth, hard-coded and empty keys | Mostly fixed in b9d3535, 2bfe1c1 | aba07a9 removes `ParseAuthorizationToken` (empty-key parser) | Library API only |
| GHSA-xpmq-2q79-5xj8 | High | Optional v4 server auth and Host-header URL | Mostly fixed in b9d3535, 2bfe1c1 | aba07a9: signed URL host no longer mutated per request and must be a bare authority | No |
| GHSA-g32g-wv53-88wv | High | LAN-reachable artifact read/write | Duplicate of f4c3; fixed in b9d3535 | aba07a9 residuals | No |
| GHSA-gfph-87pj-r593 | Medium | Hard-coded 4-byte HMAC key | Key randomized in b9d3535 | aba07a9: signed URL expiry capped at 60 minutes | No |
| GHSA-mx37-h23p-g4pp | Medium | V4 hard-coded key, unauthenticated management | Duplicate of 733g; fixed in b9d3535 | aba07a9 residuals | No |
| GHSA-chqq-hw7w-p5jg | Medium | Cross-run artifact overwrite | Fixed in 2bfe1c1 (run id bound by middleware) | `validateRunIDV4` still has its check commented out; harmless while middleware binds the run | No |
| GHSA-2w9c-j4x8-m2qc | Medium | `safeResolve` follows pre-existing symlinks | Exploitable when storage overlaps attacker content | aba07a9 (artifacts), a338915 (cache) via `os.Root` | No |
| GHSA-5693-66xq-c684 | Low | Terminal escape injection from step output | Exploitable | dabd831: strip OSC/DCS/APC/PM/SOS, non-SGR CSI, C0/C1, DEL; keep colors | No |
| GHSA-8jv2-fj8w-xcx6 | Medium | Outdated go-git | Fixed in e1cd3ee (v0.2.90) | None needed | No |
| GHSA-66w9-v53h-69cg | Medium | Incomplete fix for CVE-2026-34041 | Duplicate of xmgr; fixed in 0c739c8 (v0.2.86) | None needed | No |
| GHSA-jm9r-836x-3vhh | Medium | Workflow pinning (published) | Fixed | Supply-chain branch continues the hardening | No |
| GHSA-xmgr-9pqc-h5vw | High | set-env/add-path (published) | Fixed in 0c739c8 | 64674e6 also blocks `NODE_OPTIONS` through `::set-env::` when enabled | No |
| GHSA-x34h-54cw-9825 | High | Cache server injection (published) | Fixed | a338915 adds symlink containment | No |
| GHSA-pc99-qmg4-rcff | High | Artifact traversal (published) | Fixed | aba07a9 adds symlink containment | No |
| AUDIT-1 (new) | Medium (assessed) | Default `.secrets`/`.env`/`.vars`/`.input` in the workdir followed symlinks to host files | Exploitable for a cloned repo | 07e853d: such files are skipped with a warning; explicit paths outside the workdir still trusted | No |
| AUDIT-2 (new) | Low (assessed) | GitHub and CodeCommit remote regexes unanchored, dots unescaped | Exploitable for slug spoofing only | 39104a5 | `www.github.com` remotes no longer match |

### Grouping

- Container options and volumes: 7x4h, v5hg, hf9j, 3m58, 45wq. One root cause (unfiltered `mergo.WithOverride` merge of parsed Docker CLI options), one policy, one fix (94e9ff0, 3b73e52).
- Artifact server: f4c3, 733g, xpmq, g32g, mx37, gfph, chqq. b9d3535 and 2bfe1c1 (v0.2.90) fixed the core. aba07a9 covers residuals. f4c3, g32g and mx37 are duplicates of 733g or xpmq.
- Storage containment: 2w9c, plus CodeQL #25–#29. aba07a9 and a338915.
- Action paths and extraction: 9j7q, x66w, plus CodeQL #30, #32–#34. b6a56a0 and 439f1b1.
- Workflow commands: jg8p (new), 66w9 (duplicate of xmgr).
- Config trust: 88g4, AUDIT-1.

### Code scanning

| Alert | Rule | Triage | Fix |
| --- | --- | --- | --- |
| #36–#39 | go/incomplete-hostname-regexp | True positive (low impact) | 39104a5 |
| #30, #33, #34 | go/zipslip, go/unsafe-unzip-symlink in `host_environment.go` | True positive (x66w) | 439f1b1 |
| #32 | go/zipslip in `step_action_remote.go` | True positive (9j7q) | b6a56a0 |
| #31 | go/zipslip in `step_action_local.go` | False positive: entries are prefix-checked and docker-mode reads stay in the container; host mode already runs steps on the host | None |
| #25–#29 | go/path-injection in `pkg/artifacts/server.go` | Lexically contained before; symlink residual (2w9c) | aba07a9 |
| #5, #22 | Scorecard Branch-Protection, Code-Review | Owner settings | See below |
| #6–#9, #13, #40 | Scorecard Token-Permissions | Fixed on the supply-chain branch, except release `gh-act` `actions: write` (needed, documented) | dbbbe9e, cd8c70b, ecf67de |
| #10 | Token-Permissions `dependabot-automerge.yml` | Accepted: job-scoped writes are required to enable auto-merge | None |
| #14–#17 | Pinned-Dependencies | Fixed except `pull.Dockerfile` (`ARG IMAGE`) | 0f4cf0c, 4abb93f |
| #21 | Security-Policy | Fixed | 50ae8bf |
| #18 | Vulnerabilities | Fixed by dependency bump | b03cb7b |
| #19, #20, #23 | SAST, Fuzzing, CII badge | Informational; CodeQL already runs. Fuzzing the expression parser and workflow parser is a reasonable follow-up | None |

## Secure defaults and opt-outs

| Change | Default | Opt-out | Who is affected |
| --- | --- | --- | --- |
| Container options policy | `auto`: restrict when `--container-daemon-socket -` is set; allow otherwise, since a mounted socket already grants host access | `--container-options-policy=allow` | Users who disabled the socket and rely on `--privileged`, host binds, or devices in `container.options` or service options |
| Project `.actrc` allowlist | Only safe flags load from `./.actrc`; other lines are skipped with a warning | `--trust-project-actrc` in `~/.actrc` or on the command line | Repos whose `.actrc` sets socket, bind, privileged, or self-hosted platforms |
| `NODE_OPTIONS` through `GITHUB_ENV` and `::set-env::` | Refused, as on GitHub-hosted runners | None (matches GitHub) | Workflows that set it via `GITHUB_ENV` (already fails on GitHub) |
| Remote action paths | `uses:` org, repo, path and `runs.*` must stay inside the action repo | None | Malformed or malicious actions only |
| Default project files through symlinks | Skipped when they resolve outside the workdir | Pass the path explicitly (`--secret-file /path`) | Repos that symlink `.secrets` elsewhere |
| `pkg/common.ParseAuthorizationToken` | Removed | Use `ParseSignedAuthorizationToken` | Library consumers |
| GitHub remote regex | Exact host match | None | Remotes on `www.github.com` |

A stricter future default: stop mounting the Docker socket into job containers unless requested. That would let `auto` restrict by default. It breaks Docker-in-Docker workflows, so it needs a release note and a flag.

## Supply chain

Code fixes on `security/supply-chain-hardening`: job-scoped token permissions, no write scopes on test jobs, MegaLinter without a token, matrix values through `env`, GoReleaser pinned to v2.19.0, build provenance attestations for release archives and checksums, Chocolatey tarball checksum and HTTPS, quoted entrypoint, Docker images pinned by digest, Dependabot cooldown and Docker ecosystem, `SECURITY.md`, and Go x/ module bumps.

Owner-only settings:

| Setting | Recommended value |
| --- | --- |
| Org two-factor requirement | Required |
| Ruleset bypass actors on `main` and tags | None, or "pull request only" mode |
| Required status checks | Pin `lint` to the GitHub Actions integration (app id 15368) |
| Pull request rules | Require code owner review and approval of the most recent push |
| Actions permissions | `allowed_actions: selected`, `sha_pinning_required: true` |
| `release` and `promote` environments | Required reviewers, `can_admins_bypass: false`, deployment branches limited to `main` and tags |
| `copilot` environment | Delete, or add protection rules |
| Releases | Enable immutable releases |
| `nektos-ci` account and tokens | Review scope and rotate |
| Codecov | Move `CODECOV_TOKEN` into an environment (or OIDC) and rotate it |
| Secret scanning | Enable non-provider patterns and validity checks; keep push protection on |

## Residual risk

- The job container mounts the host Docker socket by default. Any workflow already has host-level access in that mode. The options policy only adds isolation when the socket is disabled.
- `trampoline.js` for cached actions is written under a `<sha>/<path>` directory relative to the working directory. The path is contained but the location is surprising.
- Local `uses: ./../x` is unchanged. In docker mode it resolves inside the container. In host mode steps already run on the host.
- `validateRunIDV4` keeps its commented-out check; the middleware binds the run id instead.
- x/crypto openpgp (GO-2026-5932) has no fixed release and is not called.
- gosec G115 findings in the copied Docker CLI code were not changed.
- Docker-backed integration tests (`TestRunEvent` and related) were not run in this environment; there is no Docker daemon. Run `make test` before release.
