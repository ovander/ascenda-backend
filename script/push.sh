#!/bin/bash
set -euo pipefail

# ==============================
# CONFIG
# ==============================
SSH_USER="olivier"
SSH_HOST="vandermoten.eu"
SSH_PORT="2222"
REMOTE="${SSH_USER}@${SSH_HOST}"

APP_NAME="ascenda"

VERSION="${1:-$(git describe --tags --always --dirty 2>/dev/null || echo "dev")}"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")"
BUILD_TIME="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

BIN_NAME="${APP_NAME}-${VERSION}"
LOCAL_BIN="/tmp/${BIN_NAME}"

REMOTE_TMP_DIR="/tmp/ascenda"
REMOTE_BIN="${REMOTE_TMP_DIR}/app"
REMOTE_MIGRATIONS="${REMOTE_TMP_DIR}/migrations"

# ==============================
# GUARD: prevent dirty release
# ==============================
if [[ "${VERSION}" == *"-dirty"* ]]; then
  echo "❌ Working tree is dirty. Commit your changes before deploying."
  exit 1
fi

# ==============================
# BUILD
# ==============================
echo "=============================="
echo "🔨 Building ${BIN_NAME}"
echo "Version: ${VERSION}"
echo "Commit: ${COMMIT}"
echo "Time: ${BUILD_TIME}"
echo "=============================="

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -ldflags="-s -w \
    -X main.version=${VERSION} \
    -X main.commit=${COMMIT} \
    -X main.buildTime=${BUILD_TIME}" \
  -o "${LOCAL_BIN}" \
  ./cmd/server

# ==============================
# VALIDATION
# ==============================
echo "🔍 Validating binary..."

if [ ! -f "${LOCAL_BIN}" ]; then
  echo "❌ Build failed"
  exit 1
fi

chmod +x "${LOCAL_BIN}"
echo "✔ Binary built: ${LOCAL_BIN}"

# ==============================
# CHECKSUM
# ==============================
echo "🔐 Generating checksum..."
CHECKSUM=$(shasum -a 256 "${LOCAL_BIN}" | awk '{print $1}')
echo "Checksum: ${CHECKSUM}"

# ==============================
# UPLOAD BINARY
# ==============================

# Ensure remote temp dir exists
ssh -p ${SSH_PORT} ${REMOTE} "mkdir -p /tmp/ascenda"

echo "📤 Uploading binary..."

scp -P ${SSH_PORT} "${LOCAL_BIN}" "${REMOTE}:${REMOTE_BIN}"

# ==============================
# VERIFY REMOTE
# ==============================
echo "🔍 Verifying remote binary..."

ssh -p ${SSH_PORT} ${REMOTE} "
ls -lh ${REMOTE_BIN}
"

# ==============================
# UPLOAD MIGRATIONS
# ==============================
echo "📁 Uploading migrations..."

ssh -p ${SSH_PORT} ${REMOTE} "rm -rf ${REMOTE_MIGRATIONS} && mkdir -p ${REMOTE_MIGRATIONS}"

rsync -az --delete -e "ssh -p ${SSH_PORT}" \
  migrations/ "${REMOTE}:${REMOTE_MIGRATIONS}/"

# ==============================
# CLEANUP LOCAL
# ==============================
echo "🧹 Cleaning local temp..."
rm -f "${LOCAL_BIN}"

# ==============================
# FINAL INSTRUCTIONS
# ==============================
echo ""
echo "=============================="
echo "✅ PUSH COMPLETE"
echo "=============================="
echo ""
echo "➡️ Next steps on VPS:"
echo ""
echo "ssh -p ${SSH_PORT} ${REMOTE}"
echo ""
echo "cd /opt/ascenda/backend"
echo ""
echo "# Deploy version"
echo "sudo ./deploy.sh ${VERSION}"
echo ""
echo "=============================="