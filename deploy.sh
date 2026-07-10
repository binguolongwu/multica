#!/usr/bin/env bash
#
# deploy.sh — Pull latest code from GitHub, build, and deploy to production.
#
# One-command production update:
#   bash deploy.sh
#
# Architecture:
#   Go backend (:8080) + Next.js frontend (:3001) behind nginx SSL (:443)
#   Source: /home/admin/multica        (git checkout, build here)
#   Target: /www/wwwroot/multica       (compiled binaries + frontend artifacts)
#   Config: /www/wwwroot/multica/.env.production
#   Site:   https://multica.binguosoft.net
#
set -euo pipefail

SOURCE_DIR="/home/admin/multica"
TARGET_DIR="/www/wwwroot/multica"
GO_BIN="/usr/local/go/bin/go"
PM2="/www/server/nodejs/v24.14.1/bin/pm2"

log() { echo "[$(date '+%H:%M:%S')] $*"; }

# ==================== STEP 1: Pull latest code ====================
log "=== Pulling latest code from GitHub ==="
cd "$SOURCE_DIR"
git fetch binguo main
LOCAL=$(git rev-parse HEAD)
REMOTE=$(git rev-parse binguo/main)

if [ "$LOCAL" = "$REMOTE" ]; then
  log "Already up to date ($LOCAL)"
else
  log "Pulling $REMOTE..."
  git pull --ff-only binguo main
fi

# ==================== STEP 2: Build Go backend ====================
log "=== Building Go backend ==="
cd "$SOURCE_DIR/server"

# Build with static linking (CGO_ENABLED=0) to avoid glibc version issues
# This ensures the binary runs on older systems without requiring specific glibc versions
CGO_ENABLED=0 $GO_BIN build -ldflags "-s -w" -o "$TARGET_DIR/server" ./cmd/server
CGO_ENABLED=0 $GO_BIN build -ldflags "-s -w" -o "$TARGET_DIR/migrate" ./cmd/migrate
CGO_ENABLED=0 $GO_BIN build -ldflags "-s -w -X main.version=$(date -u '+%Y.%m.%d%H%M')" -o "$TARGET_DIR/multica" ./cmd/multica
log "Backend + CLI built (static linking)"

# ==================== STEP 3: Database migrations ====================
log "=== Running migrations ==="
export $(grep -v '^#' "$TARGET_DIR/.env.production" | xargs)
"$TARGET_DIR/migrate" up || true
log "Migrations done"

# ==================== STEP 4: Build frontend ====================
log "=== Installing frontend deps ==="
cd "$SOURCE_DIR"
pnpm install --frozen-lockfile

log "=== Building Next.js frontend ==="
export REMOTE_API_URL=http://localhost:8080
export NEXT_PUBLIC_APP_VERSION=prod
export STANDALONE=true
export NODE_OPTIONS="--max-old-space-size=2048"

# Build with Turbopack (more memory-efficient than webpack on this machine).
# TypeScript checking is skipped during build to avoid OOM; run pnpm typecheck separately.
pnpm --filter=@multica/web exec next build 2>&1 | tail -5

log "=== Deploying frontend artifacts ==="
rm -rf "$TARGET_DIR/frontend"
mkdir -p "$TARGET_DIR/frontend/apps/web/.next"
cp -r "$SOURCE_DIR/apps/web/.next/standalone"/* "$TARGET_DIR/frontend/"
cp -r "$SOURCE_DIR/apps/web/.next/static" "$TARGET_DIR/frontend/apps/web/.next/"
cp -r "$SOURCE_DIR/apps/web/public" "$TARGET_DIR/frontend/apps/web/"
# Make CLI + update script available for download
mkdir -p "$TARGET_DIR/frontend/apps/web/public/downloads"
cp "$TARGET_DIR/multica" "$TARGET_DIR/frontend/apps/web/public/downloads/multica"
chmod +x "$TARGET_DIR/frontend/apps/web/public/downloads/multica"
cp "$SOURCE_DIR/update-daemon.sh" "$TARGET_DIR/frontend/apps/web/public/downloads/update-daemon.sh" 2>/dev/null || true
chmod +x "$TARGET_DIR/frontend/apps/web/public/downloads/update-daemon.sh" 2>/dev/null || true

# ==================== STEP 5: Copy configs ====================
cp -r "$SOURCE_DIR/server/migrations" "$TARGET_DIR/"
cp "$TARGET_DIR/.env.production" "$TARGET_DIR/frontend/.env" 2>/dev/null || true

# ==================== STEP 6: Regenerate pm2 ecosystem ====================
cat > "$TARGET_DIR/ecosystem.config.cjs" <<'ECONF'
const fs = require("fs");
const prodEnv = {};
const envContent = fs.readFileSync("/www/wwwroot/multica/.env.production", "utf8");
envContent.split("\n")
  .filter(line => line && !line.startsWith("#"))
  .forEach(line => {
    const eq = line.indexOf("=");
    if (eq > 0) prodEnv[line.slice(0, eq)] = line.slice(eq + 1);
  });

module.exports = {
  apps: [
    {
      name: "multica-backend",
      cwd: "/www/wwwroot/multica",
      script: "/www/wwwroot/multica/server",
      env: prodEnv,
      max_memory_restart: "512M",
      log_date_format: "YYYY-MM-DD HH:mm:ss",
      error_file: "/www/wwwroot/multica/logs/backend-error.log",
      out_file: "/www/wwwroot/multica/logs/backend-out.log",
    },
    {
      name: "multica-frontend",
      cwd: "/www/wwwroot/multica/frontend",
      script: "node",
      args: "apps/web/server.js",
      env: {
        NODE_ENV: "production",
        PORT: "3001",
        HOSTNAME: "127.0.0.1",
      },
      max_memory_restart: "512M",
      log_date_format: "YYYY-MM-DD HH:mm:ss",
      error_file: "/www/wwwroot/multica/logs/frontend-error.log",
      out_file: "/www/wwwroot/multica/logs/frontend-out.log",
    },
  ],
};
ECONF

mkdir -p "$TARGET_DIR/logs"

# ==================== STEP 7: Restart services ====================
log "=== Restarting services ==="
cd "$TARGET_DIR"

# Rebuild backend on config change (pm2 restart doesn't pick up new env vars)
$PM2 delete multica-backend 2>/dev/null || true
$PM2 start ecosystem.config.cjs --only multica-backend

if $PM2 list 2>/dev/null | grep -q "multica-frontend"; then
  $PM2 restart multica-frontend
else
  $PM2 start ecosystem.config.cjs --only multica-frontend
fi

$PM2 save

# ==================== DONE ====================
log "=== Deploy complete ==="
log "Site: https://multica.binguosoft.net"
log "Status:  $PM2 status"
log "Logs:    $PM2 logs multica-backend"
echo ""
echo "Quick checks:"
echo "  curl -sk https://multica.binguosoft.net | head"
echo "  pm2 status"
