#!/usr/bin/env bash
# Gets the running stack ready for hands-on work in the browser: builds the plugin (skip with
# --no-build), creates the admin and a team, starts fake-zoom, and installs the plugin with the
# same settings as the Playwright suite (support/plugin.ts). Rerun it after changing the plugin,
# after `make e2e-env-down`, or after running the tests (they need fake-zoom stopped first).
# Start the stack first: make e2e-env-up.
set -euo pipefail

cd "$(dirname "$0")/.."

# Playwright's default admin (testConfig in @mattermost/playwright-lib), so the suite and this
# script share the account.
ADMIN_USER=sysadmin
ADMIN_PASSWORD='Sys@dmin-sample1'
TEAM=agent

# Prints a prefixed status message.
log() { echo "[dev-setup] $*"; }
# Runs docker compose for the E2E stack.
compose() { (cd e2e/playwright && docker compose "$@"); }
# Runs mmctl in local mode inside the Mattermost container.
mmctl() { compose exec -T mattermost /mattermost/bin/mmctl --local "$@"; }

if [[ "${1:-}" != "--no-build" ]]; then
    MM_SERVICESETTINGS_ENABLEDEVELOPER=true make dist
fi

if ! mmctl user search "$ADMIN_USER" >/dev/null 2>&1; then
    log "Creating $ADMIN_USER"
    mmctl user create --email "$ADMIN_USER@sample.mattermost.com" --username "$ADMIN_USER" \
        --password "$ADMIN_PASSWORD" --system-admin --email-verified >/dev/null
fi
if ! mmctl team search "$TEAM" 2>/dev/null | grep -q "^$TEAM:"; then
    log "Creating team $TEAM"
    mmctl team create --name "$TEAM" --display-name Agent >/dev/null
fi
mmctl team users add "$TEAM" "$ADMIN_USER" >/dev/null 2>&1 || true

.cursor/fake-zoom.sh start

# Reinstalling restarts the plugin process, so it also picks up a regenerated fake-zoom certificate.
log "Installing the plugin"
(cd e2e/playwright && ADMIN_USER="$ADMIN_USER" ADMIN_PASSWORD="$ADMIN_PASSWORD" tsx --eval "
    import {Client4} from '@mattermost/client';
    import {mattermostURL} from './support/constants';
    import {installPlugin} from './support/plugin';

    const client = new Client4();
    client.setUrl(mattermostURL);
    client.login(process.env.ADMIN_USER, process.env.ADMIN_PASSWORD).
        then(() => installPlugin(client)).
        catch((err) => {
            console.error(err);
            process.exit(1);
        });
")

log "Ready: http://127.0.0.1:8065 (log in as $ADMIN_USER / $ADMIN_PASSWORD, team $TEAM)."
