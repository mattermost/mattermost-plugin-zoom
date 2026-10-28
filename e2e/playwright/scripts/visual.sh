#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

PW_VERSION="$(node -p "require('@playwright/test/package.json').version")"
export PW_VERSION
docker compose --profile visual up -d --wait visual-browser

PW_VISUAL_BROWSER_WS="ws://127.0.0.1:${PW_BROWSER_PORT:-3333}/" npx playwright test --grep @visual "$@"
