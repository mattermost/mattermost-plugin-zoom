#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

PW_VERSION="$(node -p "require('@playwright/test/package.json').version")"
export PW_VERSION
# Baselines are tied to this exact server build; bump it together with test:visual-update-snapshots.
export MM_IMAGE=mattermost/mattermost-enterprise-edition:12.0.1
./scripts/fake_zoom_cert.sh
docker compose --profile visual up -d --wait

PW_VISUAL_BROWSER_WS="ws://127.0.0.1:${PW_BROWSER_PORT:-3333}/" npx playwright test --grep @visual "$@"
