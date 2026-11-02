#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

PW_VERSION="$(node -p "require('@playwright/test/package.json').version")"
export PW_VERSION
# Baselines are tied to this exact server build; bump it together with test:visual-update-snapshots.
export MM_IMAGE=mattermost/mattermost-enterprise-edition:12.0.1

# A separate, throwaway stack: the pinned server must not start on a database another image already migrated.
export COMPOSE_PROJECT_NAME=zoom-plugin-e2e-visual
export MM_PORT="${VISUAL_MM_PORT:-8075}"
export PW_BROWSER_PORT="${VISUAL_BROWSER_PORT:-3334}"
trap 'docker compose --profile visual down -v >/dev/null 2>&1' EXIT

./scripts/fake_zoom_cert.sh
docker compose --profile visual up -d --wait

PW_VISUAL_BROWSER_WS="ws://127.0.0.1:${PW_BROWSER_PORT}/" npx playwright test --grep @visual "$@"
