#!/usr/bin/env bash
# Publish a nektos/act release to Winget without fast-forwarding the fork.
#
# komac update --submit (https://github.com/russellbanks/Komac#komac-update) opens
# the winget-pkgs pull request by creating a branch whose tip starts at
# microsoft/winget-pkgs' default-branch HEAD. Komac's add_version does that with
# createRef and winget_pkgs.default_branch_oid, not the fork's default branch.
# GitHub only accepts that SHA once it is reachable from the fork, which is why
# the old step ran `gh repo sync` (merge-upstream) against cplee/winget-pkgs master.
#
# merge-upstream is refused when the commits being merged change
# .github/workflows and the token lacks the workflow scope. The same scope is
# required to point any fork ref at a commit that introduces workflow-file
# contents not already on a branch in the fork:
# https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/scopes-for-oauth-apps#available-scopes
# https://github.com/cli/cli/issues/7574
#
# This script never calls sync-fork or merge-upstream, and it never updates the
# fork's default branch. It asks komac only to write manifests
# (`update --exit --output`, added in komac 2.17.2; --exit conflicts with
# --submit). It then tries to create the manifest branch at upstream HEAD. When
# GitHub rejects that, it commits the same manifests onto the fork's current
# default branch, which already contains its workflow files, so the new commit
# does not need the workflow scope.
set -euo pipefail

TAG="${TAG:?}"
RELEASE_REPO="${RELEASE_REPO:?}"
WINGET_FORK="${WINGET_FORK:?}"
WINGET_UPSTREAM="${WINGET_UPSTREAM:?}"
PACKAGE_ID="${PACKAGE_ID:?}"

if [[ ! "$TAG" =~ ^v[0-9][0-9A-Za-z._+-]*$ ]]; then
  echo "tag must match ^v[0-9] and contain only [0-9A-Za-z._+-]" >&2
  exit 1
fi
if [ "$(uname -m)" != "x86_64" ]; then
  echo "this script pins the x86_64 komac build; this runner is $(uname -m)" >&2
  exit 1
fi

version="${TAG#v}"
fork_owner="${WINGET_FORK%%/*}"
komac_version=2.17.2
komac_archive="komac-${komac_version}-x86_64-unknown-linux-gnu.tar.gz"
komac_sha256=b03b020eec98acfb0b53e284a6e21815b884be33de6785b97e7a120f0e748966

first=$(printf '%s' "$PACKAGE_ID" | cut -c1 | tr '[:upper:]' '[:lower:]')
manifest_dir="manifests/${first}/$(printf '%s' "$PACKAGE_ID" | tr '.' '/')/${version}"

# Branch listing is current immediately. Issue search can lag behind a pull
# request that was just opened, which would make a rerun publish a duplicate.
ref_prefix=$(jq -nr --arg name "${PACKAGE_ID}-${version}-" '$name | @uri')
refs=$(mktemp)
gh api "repos/${WINGET_FORK}/git/matching-refs/heads/${ref_prefix}" --jq '.[].ref' >"$refs"
existing=""
while IFS= read -r ref; do
  [ -n "$ref" ] || continue
  ref_name="${ref#refs/heads/}"
  # matching-refs is a prefix search, so 0.2.98 also matches 0.2.98-rc1-<uuid>.
  # Branches this script creates end in a 32-character uppercase UUID.
  suffix="${ref_name#"${PACKAGE_ID}-${version}-"}"
  if [ "$suffix" = "$ref_name" ] || [[ ! "$suffix" =~ ^[0-9A-F]{32}$ ]]; then
    continue
  fi
  existing=$(
    gh api "repos/${WINGET_UPSTREAM}/pulls?state=open&head=${fork_owner}:${ref_name}" \
      --jq '.[0].html_url'
  )
  if [ -n "$existing" ] && [ "$existing" != "null" ]; then
    break
  fi
  existing=""
done <"$refs"
rm -f "$refs"
if [ -n "$existing" ]; then
  echo "Open pull request already exists: ${existing}"
  exit 0
fi

assets=$(mktemp)
urlfile=$(mktemp)
gh release view "$TAG" --repo "$RELEASE_REPO" --json assets >"$assets"
jq -r '.assets | sort_by(.name) | .[] | select(.name | test("_Windows_[A-Za-z0-9_]+\\.zip$")) | .url' "$assets" >"$urlfile"
mapfile -t urls <"$urlfile"
rm -f "$assets" "$urlfile"
if [ "${#urls[@]}" -eq 0 ]; then
  echo "no release assets for ${TAG} match _Windows_\\w+\\.zip$" >&2
  exit 1
fi
for url in "${urls[@]}"; do
  case "$url" in
    https://github.com/"${RELEASE_REPO}"/releases/download/"${TAG}"/*.zip) ;;
    *)
      echo "refusing unexpected installer URL: ${url}" >&2
      exit 1
      ;;
  esac
done

work=$(mktemp -d)
branch=""
created_ref=0
published=0
cleanup() {
  if [ "$created_ref" = 1 ] && [ "$published" != 1 ] && [ -n "$branch" ]; then
    gh api --method DELETE "repos/${WINGET_FORK}/git/refs/heads/${branch}" >/dev/null 2>&1 || true
  fi
  rm -rf "$work"
}
trap cleanup EXIT
curl -fsSL -o "${work}/${komac_archive}" \
  "https://github.com/russellbanks/Komac/releases/download/v${komac_version}/${komac_archive}"
echo "${komac_sha256}  ${work}/${komac_archive}" | sha256sum -c -
tar -xzf "${work}/${komac_archive}" -C "$work" komac

out="${work}/manifests"
# --exit writes manifests and does not call createRef. --skip-pr-check keeps
# CI from stopping on an interactive "a pull request already exists" prompt.
"${work}/komac" update "$PACKAGE_ID" \
  --version "$version" \
  --urls "${urls[@]}" \
  --exit \
  --output "$out" \
  --skip-pr-check \
  --created-with "nektos/act"

entries="${work}/entries.json"
echo '[]' >"$entries"
while IFS= read -r -d '' file; do
  rel="${file#"${out}/"}"
  case "$rel" in
    manifests/*.yaml) ;;
    *)
      echo "refusing to publish unexpected path: ${rel}" >&2
      exit 1
      ;;
  esac
  case "$rel" in
    *..*)
      echo "refusing path: ${rel}" >&2
      exit 1
      ;;
  esac
  blob=$(
    jq -n --rawfile content "$file" '{content:$content}' |
      gh api --method POST "repos/${WINGET_FORK}/git/blobs" --input - --jq .sha
  )
  jq --arg path "$rel" --arg sha "$blob" \
    '. + [{path:$path, mode:"100644", type:"blob", sha:$sha}]' \
    "$entries" >"${entries}.next"
  mv "${entries}.next" "$entries"
done < <(find "$out" -type f -name '*.yaml' -print0)

count=$(jq 'length' "$entries")
if [ "$count" -lt 3 ]; then
  echo "komac wrote ${count} manifests under ${out}; expected the installer, locale, and version files" >&2
  exit 1
fi

if gh api "repos/${WINGET_UPSTREAM}/contents/${manifest_dir}" --jq '.[0].name' >/dev/null 2>&1; then
  state="Update version"
else
  state="New version"
fi
title="${state}: ${PACKAGE_ID} version ${version}"
body="### Pull request has been created with [komac](https://github.com/russellbanks/Komac) v${komac_version} :rocket:"

upstream_branch=$(gh api "repos/${WINGET_UPSTREAM}" --jq .default_branch)
fork_branch=$(gh api "repos/${WINGET_FORK}" --jq .default_branch)
upstream_sha=$(gh api "repos/${WINGET_UPSTREAM}/commits/${upstream_branch}" --jq .sha)
fork_sha=$(gh api "repos/${WINGET_FORK}/commits/${fork_branch}" --jq .sha)
upstream_owner="${WINGET_UPSTREAM%%/*}"
upstream_name="${WINGET_UPSTREAM#*/}"

uuid=$(tr -d '-' </proc/sys/kernel/random/uuid | tr '[:lower:]' '[:upper:]')
branch="${PACKAGE_ID}-${version}-${uuid}"
parent="$fork_sha"
base_tree=$(gh api "repos/${WINGET_FORK}/git/commits/${parent}" --jq .tree.sha)
branched_from_upstream=0

if [ "$upstream_sha" != "$fork_sha" ]; then
  # createRef at upstream HEAD is what komac --submit does. It needs no workflow
  # scope when those workflow files are already on the fork. When they are not,
  # GitHub rejects it and the manifests are committed onto the fork's default
  # branch instead, which this script does not move.
  if gh api --method POST "repos/${WINGET_FORK}/git/refs" \
    -f ref="refs/heads/${branch}" \
    -f sha="$upstream_sha" >"${work}/createref.out" 2>"${work}/createref.err"; then
    created_ref=1
    parent="$upstream_sha"
    base_tree=$(gh api "repos/${WINGET_FORK}/git/commits/${parent}" --jq .tree.sha)
    branched_from_upstream=1
    echo "Created ${branch} at ${WINGET_UPSTREAM} ${upstream_branch} ${upstream_sha}"
  else
    echo "Could not create ${branch} at upstream ${upstream_sha}."
    cat "${work}/createref.out" "${work}/createref.err" >&2 || true
    # Fork network objects are visible through the parent repo, so a commit
    # lookup cannot tell a diverged fork from an ancestor. aheadBy is 0 only
    # when every fork commit is already in upstream.
    ahead=$(
      # shellcheck disable=SC2016 # GraphQL variables are not shell variables.
      gh api graphql \
        -f query='query($owner:String!, $name:String!, $ref:String!, $sha:String!) {
          repository(owner:$owner, name:$name) {
            ref(qualifiedName:$ref) {
              compare(headRef:$sha) { aheadBy }
            }
          }
        }' \
        -f owner="$upstream_owner" \
        -f name="$upstream_name" \
        -f ref="refs/heads/${upstream_branch}" \
        -f sha="$fork_sha" \
        --jq '.data.repository.ref.compare.aheadBy'
    )
    if [ "$ahead" != "0" ]; then
      echo "${WINGET_FORK} ${fork_branch} (${fork_sha}) is not an ancestor of ${WINGET_UPSTREAM} ${upstream_branch}; refusing to publish fork-only commits" >&2
      exit 1
    fi
    echo "Committing the manifests onto ${WINGET_FORK} ${fork_branch} (${fork_sha}) instead, without updating that branch."
  fi
fi

tree_sha=$(
  jq -n --arg base "$base_tree" --slurpfile tree "$entries" '{base_tree:$base, tree:$tree[0]}' |
    gh api --method POST "repos/${WINGET_FORK}/git/trees" --input - --jq .sha
)
commit_sha=$(
  jq -n --arg message "$title" --arg tree "$tree_sha" --arg parent "$parent" \
    '{message:$message, tree:$tree, parents:[$parent]}' |
    gh api --method POST "repos/${WINGET_FORK}/git/commits" --input - --jq .sha
)

if [ "$branched_from_upstream" = 1 ]; then
  gh api --method PATCH "repos/${WINGET_FORK}/git/refs/heads/${branch}" -f sha="$commit_sha" >/dev/null
else
  gh api --method POST "repos/${WINGET_FORK}/git/refs" \
    -f ref="refs/heads/${branch}" \
    -f sha="$commit_sha" >/dev/null
  created_ref=1
fi

pr_url=$(
  jq -n --arg title "$title" --arg head "${fork_owner}:${branch}" --arg base "$upstream_branch" --arg body "$body" \
    '{title:$title, head:$head, base:$base, body:$body}' |
    gh api --method POST "repos/${WINGET_UPSTREAM}/pulls" --input - --jq .html_url
)
published=1
echo "Pull request: ${pr_url}"
