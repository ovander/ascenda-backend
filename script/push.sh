#!/usr/bin/env bash
# =============================================================================
# push.sh  —  Ascenda backend build, upload & deploy to the VPS
#
# Reference model: vandermoten.eu · Multi-App VPS · April 2026
#
# Usage:
#   ./script/push.sh [version]              # build, upload, deploy
#   ./script/push.sh [version] --no-deploy  # build and upload only
#
#   version  a vX.Y.Z tag on HEAD; without it, the v* tag on HEAD is used
#            (see script/version-guard.sh — it refuses a dirty or untagged tree)
#
# What it does:
#   1. Resolve and check the version (script/version-guard.sh)
#   2. Build a static linux binary for the VPS's CPU, stamped with the version
#   3. Upload app, migrations/*.sql and VERSION to /tmp/ascenda-backend
#   4. Run /opt/apps/ascenda/deploy-backend.sh <version> on the VPS
#      (sudo may ask for your password)
#
# --no-deploy stops after step 3: use it to inspect the upload first, e.g.
# to dry-run a migration against the production database, then deploy with
#   ssh -p "$SSH_PORT" "$SSH_USER@$SSH_HOST"
#   sudo /opt/apps/ascenda/deploy-backend.sh <version>
#
# The VPS address is not kept in the repository. Set SSH_USER, SSH_HOST and
# SSH_PORT in the environment or in ~/.config/ascenda/deploy.env (another file
# with ASCENDA_DEPLOY_ENV), for example:
#   SSH_USER=deploy
#   SSH_HOST=vps.example.com
#   SSH_PORT=22
# =============================================================================
set -euo pipefail

# ── Configuration ─────────────────────────────────────────────────────────────
DEPLOY_ENV="${ASCENDA_DEPLOY_ENV:-${HOME}/.config/ascenda/deploy.env}"
# shellcheck source=/dev/null
[ -f "${DEPLOY_ENV}" ] && . "${DEPLOY_ENV}"

REMOTE_TMP_DIR="/tmp/ascenda-backend"
REMOTE_DEPLOY="/opt/apps/ascenda/deploy-backend.sh"
BUILDINFO="github.com/ovander/backendkit/buildinfo"

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# ── Arguments ─────────────────────────────────────────────────────────────────
DEPLOY=true
REQUESTED=""
for arg in "$@"; do
  case "${arg}" in
    --no-deploy) DEPLOY=false ;;
    -h|--help)   sed -n '2,/^# ====/p' "$0"; exit 0 ;;
    -*)          echo "❌ Unknown option: ${arg}" >&2; exit 1 ;;
    *)           REQUESTED="${arg}" ;;
  esac
done

# ── VPS address (after the arguments, so --help works without it) ───────────
for var in SSH_USER SSH_HOST SSH_PORT; do
  if [ -z "${!var:-}" ]; then
    echo "❌ ${var} is not set: define SSH_USER, SSH_HOST and SSH_PORT in ${DEPLOY_ENV} (see the header of this script)" >&2
    exit 1
  fi
done
REMOTE="${SSH_USER}@${SSH_HOST}"
SSH_OPTS=(-p "${SSH_PORT}" -o StrictHostKeyChecking=accept-new)

# ── 1. Version ────────────────────────────────────────────────────────────────
VERSION="$(bash "${REPO_ROOT}/script/version-guard.sh" "${REPO_ROOT}" "${REQUESTED}")"
COMMIT="$(git -C "${REPO_ROOT}" rev-parse --short HEAD)"
BUILD_TIME="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

# ── 2. Build for the VPS's CPU ────────────────────────────────────────────────
REMOTE_ARCH="$(ssh "${SSH_OPTS[@]}" "${REMOTE}" uname -m)"
case "${REMOTE_ARCH}" in
  x86_64)        GOARCH=amd64 ;;
  aarch64|arm64) GOARCH=arm64 ;;
  *) echo "❌ Unsupported VPS architecture: ${REMOTE_ARCH}" >&2; exit 1 ;;
esac

echo "=============================="
echo "🔨 Building Ascenda backend ${VERSION}"
echo "Commit:   ${COMMIT}"
echo "Target:   linux/${GOARCH} (${REMOTE}, ${REMOTE_ARCH})"
echo "Time:     ${BUILD_TIME}"
echo "=============================="

STAGE="$(mktemp -d)"
trap 'rm -rf "${STAGE}"' EXIT

(
  cd "${REPO_ROOT}"
  CGO_ENABLED=0 GOOS=linux GOARCH="${GOARCH}" go build \
    -ldflags "-s -w \
      -X ${BUILDINFO}.Version=${VERSION} \
      -X ${BUILDINFO}.GitCommit=${COMMIT} \
      -X ${BUILDINFO}.BuildTime=${BUILD_TIME}" \
    -o "${STAGE}/app" \
    ./cmd/server
)
chmod +x "${STAGE}/app"

# The migrations are embedded in the binary; the .sql copies are uploaded so
# they can be read or dry-run on the VPS before deploying.
mkdir -p "${STAGE}/migrations"
cp "${REPO_ROOT}"/migrations/*.sql "${STAGE}/migrations/"
echo "${VERSION}" > "${STAGE}/VERSION"

CHECKSUM="$(shasum -a 256 "${STAGE}/app" | awk '{ print $1 }')"
echo "✔ Binary built ($(du -h "${STAGE}/app" | awk '{ print $1 }'), sha256 ${CHECKSUM:0:16}…)"
echo "✔ $(find "${STAGE}/migrations" -name '*.sql' | wc -l | tr -d ' ') migration files staged"

# ── 3. Upload ─────────────────────────────────────────────────────────────────
echo "📤 Uploading → ${REMOTE}:${REMOTE_TMP_DIR}/"
ssh "${SSH_OPTS[@]}" "${REMOTE}" "rm -rf ${REMOTE_TMP_DIR} && mkdir -p ${REMOTE_TMP_DIR}"
rsync -az -e "ssh ${SSH_OPTS[*]}" "${STAGE}/" "${REMOTE}:${REMOTE_TMP_DIR}/"

REMOTE_CHECKSUM="$(ssh "${SSH_OPTS[@]}" "${REMOTE}" "sha256sum ${REMOTE_TMP_DIR}/app" | awk '{ print $1 }')"
if [ "${REMOTE_CHECKSUM}" != "${CHECKSUM}" ]; then
  echo "❌ Upload corrupted: remote sha256 ${REMOTE_CHECKSUM} ≠ local ${CHECKSUM}" >&2
  exit 1
fi
echo "✔ Upload verified"

# The VPS runs its own copy of deploy-backend.sh; say so when it is not this one.
LOCAL_DEPLOY_SUM="$(shasum -a 256 "${REPO_ROOT}/script/deploy-backend.sh" | awk '{ print $1 }')"
REMOTE_DEPLOY_SUM="$(ssh "${SSH_OPTS[@]}" "${REMOTE}" "sha256sum ${REMOTE_DEPLOY} 2>/dev/null" | awk '{ print $1 }' || true)"
if [ "${LOCAL_DEPLOY_SUM}" != "${REMOTE_DEPLOY_SUM}" ]; then
  echo "⚠️  ${REMOTE_DEPLOY} differs from script/deploy-backend.sh — to install this one:"
  echo "    scp -P ${SSH_PORT} script/deploy-backend.sh ${REMOTE}:/tmp/deploy-backend.sh"
  echo "    ssh -t -p ${SSH_PORT} ${REMOTE} 'sudo install -m 755 /tmp/deploy-backend.sh ${REMOTE_DEPLOY}'"
fi

# ── 4. Deploy ─────────────────────────────────────────────────────────────────
if [ "${DEPLOY}" = false ]; then
  echo ""
  echo "=============================="
  echo "✅ ${VERSION} uploaded (not deployed)"
  echo ""
  echo "➡️  To deploy:"
  echo "    ssh -p ${SSH_PORT} ${REMOTE}"
  echo "    sudo ${REMOTE_DEPLOY} ${VERSION}"
  echo "=============================="
  exit 0
fi

echo "🚀 Deploying on the VPS..."
ssh -t "${SSH_OPTS[@]}" "${REMOTE}" "sudo ${REMOTE_DEPLOY} ${VERSION}"
