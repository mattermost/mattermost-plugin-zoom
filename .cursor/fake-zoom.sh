#!/usr/bin/env bash
# Runs e2e/playwright/fake-zoom in the background for hands-on work, with the same ports, URLs and
# OAuth app the Playwright suite uses (global_setup.ts). The suite starts its own fake on those
# ports, so stop this one before `make e2e`.
#   .cursor/fake-zoom.sh start|stop|restart|status
set -euo pipefail

cd "$(dirname "$0")/../e2e/playwright"

# Prints KEY's last value from .env (read, not sourced), the file support/constants.ts loads.
env_file_value() {
    [[ -f .env ]] || return 0
    node -e 'const v = require("node:util").parseEnv(require("node:fs").readFileSync(".env", "utf8"))[process.argv[1]]; if (v !== undefined) console.log(v);' "$1"
}

# Like process.loadEnvFile, .env never overrides variables already set in the environment.
PORT="${FAKE_ZOOM_PORT:-$(env_file_value FAKE_ZOOM_PORT)}"
PORT="${PORT:-8091}"
TLS_PORT="${FAKE_ZOOM_TLS_PORT:-$(env_file_value FAKE_ZOOM_TLS_PORT)}"
TLS_PORT="${TLS_PORT:-8444}"
PID_FILE=/tmp/fake-zoom.pid
LOG_FILE=/tmp/fake-zoom.log

# Prints a prefixed status message.
log() { echo "[fake-zoom] $*"; }

# Prints running, stopped (connection refused) or unresponsive (anything else, e.g. a timeout).
state() {
    local rc=0
    curl -fsS --max-time 2 "http://127.0.0.1:$PORT/__control/requests" >/dev/null 2>&1 || rc=$?
    case "$rc" in
        0) echo running ;;
        7) echo stopped ;;
        *) echo unresponsive ;;
    esac
}

# Succeeds when fake-zoom answers on its control endpoint.
running() { [[ "$(state)" == running ]]; }

# Starts fake-zoom in the background and waits for it to answer.
start() {
    case "$(state)" in
        running)
            log "Already running on :$PORT and :$TLS_PORT."
            return 0
            ;;
        unresponsive)
            log "A listener on :$PORT is not responding; run stop or free the port first." >&2
            exit 1
            ;;
    esac
    # Same certificate the stack mounts into Mattermost's trusted roots; a no-op while it is valid.
    ./scripts/fake_zoom_cert.sh
    setsid nohup tsx --eval "
        import {startFakeZoom} from './fake-zoom/server';
        import {certDir, fakeZoomAPIURL, fakeZoomDownloadURL, fakeZoomPort, fakeZoomTLSPort, fakeZoomURL, mattermostURL, oauthClientId, oauthClientSecret, pluginId} from './support/constants';
        startFakeZoom({
            port: fakeZoomPort,
            tlsPort: fakeZoomTLSPort,
            certDir,
            zoomURL: fakeZoomURL,
            apiURL: fakeZoomAPIURL,
            downloadURL: fakeZoomDownloadURL,
            clientId: oauthClientId,
            clientSecret: oauthClientSecret,
            redirectURI: mattermostURL + '/plugins/' + pluginId + '/oauth2/complete',
        });
    " >"$LOG_FILE" 2>&1 &
    echo $! >"$PID_FILE"
    for _ in $(seq 1 30); do
        if running; then
            log "Running on :$PORT and :$TLS_PORT (logs: $LOG_FILE)."
            return 0
        fi
        sleep 1
    done
    log "Did not start within 30 seconds:" >&2
    cat "$LOG_FILE" >&2
    exit 1
}

# Kills the background process group and waits until nothing listens on the port.
stop() {
    local current
    if [[ -f "$PID_FILE" ]]; then
        kill -- "-$(cat "$PID_FILE")" 2>/dev/null || true
        rm -f "$PID_FILE"
    fi
    for _ in $(seq 1 10); do
        current="$(state)"
        [[ "$current" == stopped ]] && { log "Stopped."; return 0; }
        sleep 1
    done
    if [[ "$current" == unresponsive ]]; then
        log "A listener on :$PORT is still active but not responding." >&2
    else
        log "Something else is still serving :$PORT." >&2
    fi
    exit 1
}

# Reports whether fake-zoom is running, stopped or unresponsive.
status() {
    case "$(state)" in
        running) log "Running on :$PORT and :$TLS_PORT." ;;
        unresponsive) log "A listener on :$PORT is not responding." >&2; exit 1 ;;
        *) log "Not running."; exit 1 ;;
    esac
}

case "${1:-start}" in
    start) start ;;
    stop) stop ;;
    restart) stop && start ;;
    status) status ;;
    *) echo "usage: $0 start|stop|restart|status" >&2; exit 2 ;;
esac
