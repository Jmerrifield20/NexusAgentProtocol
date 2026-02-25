#!/usr/bin/env bash
# deploy.sh — Full deployment script for the NAP registry VM
#
# Run this on the VM after SSH-ing in:
#   cd ~/NexusAgentProtocol && bash scripts/deploy.sh
#
# Flags:
#   --skip-migrate      Skip DB migrations (use for code-only deploys)
#   --skip-frontend     Skip Next.js rebuild
#   --api-only          Rebuild + restart API only (same as --skip-frontend)
#
# Required env var for migrations:
#   export DATABASE_URL='postgres://nexus:PASS@DB_IP/nexus?sslmode=require'
#
# The DATABASE_URL value is stored in GCP Secret Manager as 'registry-db-url'.
# Retrieve it with:
#   export DATABASE_URL=$(gcloud secrets versions access latest --secret=registry-db-url)

set -euo pipefail

# ── Config ────────────────────────────────────────────────────────────────────
REPO_DIR="${REPO_DIR:-$HOME/NexusAgentProtocol}"
REGISTRY_BINARY=/usr/local/bin/nap-registry
MIGRATE_BINARY=/usr/local/bin/nap-migrate
FRONTEND_REGISTRY_URL="${NEXT_PUBLIC_REGISTRY_URL:-https://api.nexusagentprotocol.com}"
API_HEALTH_URL="https://api.nexusagentprotocol.com/api/v1/agents"

# ── Flags ─────────────────────────────────────────────────────────────────────
SKIP_MIGRATE=0
SKIP_FRONTEND=0

for arg in "$@"; do
  case $arg in
    --skip-migrate)          SKIP_MIGRATE=1 ;;
    --skip-frontend|--api-only) SKIP_FRONTEND=1 ;;
    --help)
      sed -n '2,15p' "$0"
      exit 0
      ;;
    *) echo "Unknown flag: $arg" >&2; exit 1 ;;
  esac
done

# ── Helpers ───────────────────────────────────────────────────────────────────
log()  { echo "[$(date '+%H:%M:%S')] ==> $*"; }
die()  { echo "[$(date '+%H:%M:%S')] ERROR: $*" >&2; exit 1; }
warn() { echo "[$(date '+%H:%M:%S')] WARNING: $*" >&2; }

log "Starting NAP deployment"
log "Repo:     $REPO_DIR"
log "Migrate:  $([ $SKIP_MIGRATE -eq 1 ] && echo skipped || echo enabled)"
log "Frontend: $([ $SKIP_FRONTEND -eq 1 ] && echo skipped || echo enabled)"
echo ""

# ── 1. Pull latest code ───────────────────────────────────────────────────────
log "Pulling latest code..."
cd "$REPO_DIR"

CURRENT_SHA=$(git rev-parse HEAD)
git pull
NEW_SHA=$(git rev-parse HEAD)

if [[ "$CURRENT_SHA" == "$NEW_SHA" ]]; then
  warn "No new commits — already up to date. Continuing anyway."
fi
log "Now at $(git log --oneline -1)"
echo ""

# ── 2. DB migrations ──────────────────────────────────────────────────────────
if [[ $SKIP_MIGRATE -eq 0 ]]; then
  [[ -z "${DATABASE_URL:-}" ]] && die "DATABASE_URL is not set.
  Export it before running this script:
    export DATABASE_URL=\$(gcloud secrets versions access latest --secret=registry-db-url)"

  log "Building migration binary..."
  go build -o "$MIGRATE_BINARY" ./cmd/migrate

  log "Running migrations..."
  "$MIGRATE_BINARY" up
  log "Migrations complete."
  echo ""
fi

# ── 3. Build + restart API ────────────────────────────────────────────────────
log "Building registry binary..."
go build -o "$REGISTRY_BINARY" ./cmd/registry

log "Restarting nap-registry..."
sudo systemctl restart nap-registry
sleep 2

if sudo systemctl is-active --quiet nap-registry; then
  log "nap-registry is running."
else
  die "nap-registry failed to start. Check logs:
    sudo journalctl -u nap-registry -n 50"
fi
echo ""

# ── 4. Build + restart frontend ───────────────────────────────────────────────
if [[ $SKIP_FRONTEND -eq 0 ]]; then
  log "Building Next.js frontend (this takes ~60s)..."
  cd "$REPO_DIR/web"
  NEXT_PUBLIC_REGISTRY_URL="$FRONTEND_REGISTRY_URL" npm run build

  log "Restarting nap-frontend..."
  sudo systemctl restart nap-frontend
  sleep 2

  if sudo systemctl is-active --quiet nap-frontend; then
    log "nap-frontend is running."
  else
    die "nap-frontend failed to start. Check logs:
    sudo journalctl -u nap-frontend -n 50"
  fi
  echo ""
fi

# ── 5. API health check ───────────────────────────────────────────────────────
log "Running API health check..."
HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$API_HEALTH_URL" 2>/dev/null || echo "000")

if [[ "$HTTP_STATUS" == "200" ]]; then
  log "Health check passed (HTTP $HTTP_STATUS) — $API_HEALTH_URL"
else
  warn "Health check returned HTTP $HTTP_STATUS — verify manually: curl -i $API_HEALTH_URL"
fi

echo ""
log "Deploy complete! Commit: $(git log --oneline -1)"
