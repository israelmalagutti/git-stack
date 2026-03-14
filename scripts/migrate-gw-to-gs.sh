#!/bin/bash
set -e

# Migrate gw (git-wrapper) config files to gs (git-stack)
# Run this inside any git repo that was previously initialized with gw.
#
# Usage: gs-migrate   (or bash scripts/migrate-gw-to-gs.sh)

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

# Must be inside a git repo
git rev-parse --git-dir >/dev/null 2>&1 || error "Not inside a git repository"

COMMON_DIR=$(git rev-parse --git-common-dir)

OLD_CONFIG="$COMMON_DIR/.gw_config"
OLD_METADATA="$COMMON_DIR/.gw_stack_metadata"
OLD_CONTINUE="$COMMON_DIR/.gw_continue_state"

NEW_CONFIG="$COMMON_DIR/.gs_config"
NEW_METADATA="$COMMON_DIR/.gs_stack_metadata"
NEW_CONTINUE="$COMMON_DIR/.gs_continue_state"

# Check if there's anything to migrate
if [ ! -f "$OLD_CONFIG" ] && [ ! -f "$OLD_METADATA" ] && [ ! -f "$OLD_CONTINUE" ]; then
    if [ -f "$NEW_CONFIG" ]; then
        info "Already using gs config files. Nothing to migrate."
    else
        warn "No gw config files found. Run 'gs init' to initialize."
    fi
    exit 0
fi

# Check for conflicts (new files already exist)
if [ -f "$NEW_CONFIG" ] || [ -f "$NEW_METADATA" ]; then
    error "gs config files already exist. Remove them first if you want to re-migrate."
fi

MIGRATED=0

if [ -f "$OLD_CONFIG" ]; then
    mv "$OLD_CONFIG" "$NEW_CONFIG"
    info "Migrated .gw_config → .gs_config"
    MIGRATED=$((MIGRATED + 1))
fi

if [ -f "$OLD_METADATA" ]; then
    mv "$OLD_METADATA" "$NEW_METADATA"
    info "Migrated .gw_stack_metadata → .gs_stack_metadata"
    MIGRATED=$((MIGRATED + 1))
fi

if [ -f "$OLD_CONTINUE" ]; then
    mv "$OLD_CONTINUE" "$NEW_CONTINUE"
    info "Migrated .gw_continue_state → .gs_continue_state"
    MIGRATED=$((MIGRATED + 1))
fi

echo ""
info "Migration complete ($MIGRATED file(s) moved)."
info "You can now use 'gs' as usual."
