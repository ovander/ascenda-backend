#!/usr/bin/env bash
# =============================================================================
# version-guard.sh  —  resolve the version a release is built and deployed as
#
# Usage:   bash script/version-guard.sh <repo-root> [version]
# Prints:  the version on stdout; every message goes to stderr.
#
# A release is named after a tag, so the build must be that tag, exactly:
#   - the working tree is clean (no modified, staged or untracked files —
#     an untracked source file is compiled in just the same);
#   - the version is a vX.Y.Z tag that points at HEAD (with no argument, the
#     one v* tag on HEAD is used);
#   - the tag is on origin, at the same commit, so the release can be found
#     and rebuilt from GitHub later.
#
# It refuses rather than guessing: a fallback such as `git describe || echo`
# always answers, including with a name that no build ever had.
#
# The same script lives in ascenda-frontend (scripts/version-guard.sh); change
# both together.
# =============================================================================
set -euo pipefail

ROOT="${1:?usage: version-guard.sh <repo-root> [version]}"
VERSION="${2:-}"

die() {
  echo "❌ $1" >&2
  shift
  for line in "$@"; do echo "   $line" >&2; done
  exit 1
}

cd "${ROOT}"
git rev-parse --git-dir >/dev/null 2>&1 || die "${ROOT} is not a git repository."

DIRTY="$(git status --porcelain)"
if [ -n "${DIRTY}" ]; then
  echo "❌ The working tree is not clean; a release must be exactly its tag:" >&2
  echo "${DIRTY}" | sed 's/^/     /' >&2
  echo "   → commit, stash or remove these files, then run again." >&2
  exit 1
fi

HEAD_SHA="$(git rev-parse HEAD)"
HEAD_SHORT="$(git rev-parse --short HEAD)"

if [ -z "${VERSION}" ]; then
  TAGS="$(git tag --points-at HEAD --list 'v*')"
  case "$(printf '%s' "${TAGS}" | grep -c . || true)" in
    0) die "HEAD (${HEAD_SHORT}) has no version tag." \
           "→ check out a release:  git checkout vX.Y.Z" \
           "→ or tag this commit:   git tag -a vX.Y.Z -m vX.Y.Z && git push origin vX.Y.Z" ;;
    1) VERSION="${TAGS}" ;;
    *) die "HEAD (${HEAD_SHORT}) has several version tags: $(echo "${TAGS}" | tr '\n' ' ')" \
           "→ name the one to release." ;;
  esac
fi

[[ "${VERSION}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]] \
  || die "'${VERSION}' is not a release version (expected vX.Y.Z)."

TAG_SHA="$(git rev-parse -q --verify "refs/tags/${VERSION}^{commit}" || true)"
[ -n "${TAG_SHA}" ] \
  || die "There is no tag ${VERSION} in this clone." \
         "→ fetch tags:  git fetch origin --tags" \
         "→ or create it on the commit to release:  git tag -a ${VERSION} -m ${VERSION}"

[ "${TAG_SHA}" = "${HEAD_SHA}" ] \
  || die "${VERSION} is commit $(git rev-parse --short "${TAG_SHA}"), but HEAD is ${HEAD_SHORT}." \
         "→ check it out:  git checkout ${VERSION}"

# An annotated tag is listed twice by ls-remote; the ^{} line is its commit.
REMOTE_REFS="$(git ls-remote --tags origin "refs/tags/${VERSION}" "refs/tags/${VERSION}^{}")" \
  || die "Could not reach origin to check that ${VERSION} is pushed."
REMOTE_SHA="$(echo "${REMOTE_REFS}" | awk '/\^\{\}$/ { print $1 }')"
[ -n "${REMOTE_SHA}" ] || REMOTE_SHA="$(echo "${REMOTE_REFS}" | awk 'NR == 1 { print $1 }')"

[ -n "${REMOTE_SHA}" ] \
  || die "Tag ${VERSION} is not on origin." \
         "→ push it first:  git push origin ${VERSION}"
[ "${REMOTE_SHA}" = "${HEAD_SHA}" ] \
  || die "Tag ${VERSION} on origin is commit ${REMOTE_SHA:0:7}, not ${HEAD_SHORT} as here." \
         "→ one of the two was moved; check which commit ${VERSION} should be."

echo "✔ ${VERSION} = ${HEAD_SHORT}, clean tree, tag on origin" >&2
echo "${VERSION}"
