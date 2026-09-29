#!/bin/bash
# =============================================================================
# deploy-backend.sh  —  Ascenda backend deployment, run on the VPS
#
# Lives at: /opt/apps/ascenda/deploy-backend.sh
# Install:  scp -P 2222 script/deploy-backend.sh olivier@vandermoten.eu:/tmp/deploy-backend.sh
#           ssh -t -p 2222 olivier@vandermoten.eu \
#               "sudo install -m 755 /tmp/deploy-backend.sh /opt/apps/ascenda/deploy-backend.sh"
#
# Usage:    sudo /opt/apps/ascenda/deploy-backend.sh [version]
#           Normally run by script/push.sh, which uploads app, migrations and
#           VERSION to /tmp/ascenda-backend first. Without an argument the
#           version is read from /tmp/ascenda-backend/VERSION.
#
# On failure the current link is pointed back at the previous release and the
# service is started again. Once migrations have run, the previous release may
# not start on the migrated schema: restore the database backup in that case.
# =============================================================================

set -euo pipefail

echo "=============================="
echo "🚀 Ascenda Backend Deployment START"
echo "Date: $(date -u)"
echo "=============================="

# -----------------------------
# CONFIG
# -----------------------------
APP_DIR="/opt/apps/ascenda"
RELEASES_DIR="$APP_DIR/releases/backend"
CURRENT_LINK="$APP_DIR/current"
MIGRATIONS_DIR="$APP_DIR/migrations"
ENV_FILE="$APP_DIR/env/.env"

TMP_DIR="/tmp/ascenda-backend"
TMP_BIN="$TMP_DIR/app"
TMP_MIGRATIONS="$TMP_DIR/migrations"

SERVICE="ascenda"
USER="olivier"

# The health check calls the port the service listens on, as set in the env
# file (PORT, default 8080 like the server). The API binds to loopback on this
# host (BIND_ADDRESS=127.0.0.1), and Socrate's admin API owns 127.0.0.1:8082.
env_value() { sed -n "s/^$1=//p" "$ENV_FILE" 2>/dev/null | tail -n 1 | tr -d "\"' "; }
API_PORT="$(env_value PORT)"
API_URL="http://127.0.0.1:${API_PORT:-8080}/health"

# -----------------------------
# VERSION
# -----------------------------
# The argument and the uploaded VERSION file (written by push.sh, which stamps
# the same version into the binary) must agree: a release directory named after
# one version holding a binary built as another makes `readlink current` lie.
# Without a VERSION file there is nothing to compare, and the argument stands.
PUSHED_VERSION="$(cat "$TMP_DIR/VERSION" 2>/dev/null || echo "")"
VERSION="${1:-$PUSHED_VERSION}"

if [ -z "$VERSION" ]; then
    echo "❌ Usage: $0 <version>   (or run push.sh first; it writes $TMP_DIR/VERSION)"
    exit 1
fi
if [ -n "$PUSHED_VERSION" ] && [ "$VERSION" != "$PUSHED_VERSION" ]; then
    echo "❌ Refusing to deploy: the version you named is not the version that was pushed."
    echo "     argument   $VERSION"
    echo "     pushed     $PUSHED_VERSION   ($TMP_DIR/VERSION)"
    echo "   → deploy what was pushed:   sudo $0 $PUSHED_VERSION"
    echo "   → or push what you meant:   ./script/push.sh $VERSION"
    exit 1
fi

RELEASE_DIR="$RELEASES_DIR/$VERSION"
MIGRATED=false

# -----------------------------
# ROLLBACK
# -----------------------------
rollback() {
    trap - ERR
    echo "❌ Deployment failed — rolling back..."

    if [ -n "${PREVIOUS:-}" ]; then
        sudo ln -sfn "$PREVIOUS" "$CURRENT_LINK"
        echo "🔗 current → $PREVIOUS"
    else
        echo "⚠️ No previous release to roll back to"
    fi

    if sudo systemctl start $SERVICE && sleep 2 && systemctl is-active --quiet $SERVICE; then
        echo "✔ $SERVICE running on the previous release"
    else
        echo "❌ $SERVICE did not start — check: sudo journalctl -u $SERVICE -n 50 --no-pager"
    fi
    if [ "$MIGRATED" = true ]; then
        echo "⚠️ The migrations of $VERSION ran, at least in part. If the previous release"
        echo "   fails on the changed schema, restore the database backup taken before this deploy."
    fi

    exit 1
}

trap rollback ERR

# -----------------------------
# PRECHECKS
# -----------------------------
echo "🔍 Pre-checks..."

[ -f "$TMP_BIN" ]       || { echo "❌ Missing binary in $TMP_BIN — run push.sh first"; exit 1; }
[ -d "$TMP_MIGRATIONS" ] || { echo "❌ Missing migrations in $TMP_MIGRATIONS"; exit 1; }

echo "✔ Pre-checks OK"

# Captured before anything changes, so that a failure at any later step
# (migrations included) returns to it and restarts the service.
PREVIOUS="$(readlink -f $CURRENT_LINK 2>/dev/null || echo "")"
echo "ℹ️  Previous release: ${PREVIOUS:-none}"

# -----------------------------
# STOP SERVICE
# -----------------------------
echo "🛑 Stopping service..."
sudo systemctl stop $SERVICE

# -----------------------------
# CREATE RELEASE
# -----------------------------
echo "📁 Creating release $VERSION..."

sudo mkdir -p "$RELEASE_DIR"
sudo mv "$TMP_BIN" "$RELEASE_DIR/app"

sudo chown -R $USER:$USER "$RELEASE_DIR"
sudo chmod +x "$RELEASE_DIR/app"

# -----------------------------
# SYNC MIGRATIONS
# -----------------------------
echo "📁 Syncing migrations..."

sudo rsync -av --delete "$TMP_MIGRATIONS/" "$MIGRATIONS_DIR/"
sudo chown -R $USER:$USER "$MIGRATIONS_DIR"
sudo chmod -R 755 "$MIGRATIONS_DIR"
sudo chmod 644 "$MIGRATIONS_DIR"/*.sql || true

# -----------------------------
# RUN MIGRATIONS
# -----------------------------
echo "🗄 Running migrations..."
MIGRATED=true   # set first: a run that fails part-way has still changed the schema

sudo -u $USER bash -c "
set -a
source $ENV_FILE
set +a
$RELEASE_DIR/app migrate
"

# -----------------------------
# SWITCH RELEASE
# -----------------------------
echo "🔁 Switching release..."

sudo ln -sfn "$RELEASE_DIR" "$CURRENT_LINK"

# -----------------------------
# START SERVICE
# -----------------------------
echo "▶️ Starting service..."
sudo systemctl start $SERVICE

# -----------------------------
# CHECK SERVICE
# -----------------------------
if ! systemctl is-active --quiet $SERVICE; then
    echo "❌ Service failed to start"
    rollback
fi

# -----------------------------
# HEALTHCHECK
# -----------------------------
echo "🌐 Checking API..."

for i in {1..10}; do
    if curl -fs $API_URL > /dev/null; then
        echo "✔ API healthy"
        break
    fi
    sleep 1
done

if ! curl -fs $API_URL > /dev/null; then
    echo "❌ API healthcheck failed"
    rollback
fi

# -----------------------------
# LOGS
# -----------------------------
echo "🔍 Service status:"
sudo systemctl status $SERVICE --no-pager

echo "📜 Recent logs:"
sudo journalctl -u $SERVICE -n 20 --no-pager

# -----------------------------
# CLEAN OLD RELEASES
# -----------------------------
echo "🧹 Cleaning old releases (keep last 5)..."

cd "$RELEASES_DIR"
ls -dt */ 2>/dev/null | tail -n +6 | xargs -r sudo rm -rf

# -----------------------------
# CLEANUP TMP
# -----------------------------
echo "🧹 Cleaning upload temp..."
rm -rf "$TMP_DIR"

# -----------------------------
# DONE
# -----------------------------
echo "=============================="
echo "✅ Deployment SUCCESS"
echo "Version: $VERSION"
echo "Active:  $(readlink -f $CURRENT_LINK)"
echo "=============================="