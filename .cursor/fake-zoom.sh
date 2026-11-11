#!/usr/bin/env bash
# Runs e2e/playwright/fake-zoom in the background for hands-on work, with the same ports, URLs and
# OAuth app the Playwright suite uses (global_setup.ts). The suite starts its own fake on those
# ports, so stop this one before `make e2e`.
#   .cursor/fake-zoom.sh start|stop|restart|status
set -euo pipefail

cd "$(dirname "$0")/../e2e/playwright"

PORT="${FAKE_ZOOM_PORT:-8091}"
TLS_PORT="${FAKE_ZOOM_TLS_PORT:-8444}"
PID_FILE=/tmp/fake-zoom.pid
LOG_FILE=/tmp/fake-zoom.log

log() { echo "[fake-zoom] $*"; }
running() { curl -fsS "http://127.0.0.1:$PORT/__control/requests" >/dev/null 2>&1; }

start() {
    if running; then
        log "Already running on :$PORT and :$TLS_PORT."
        return 0
    fi
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

stop() {
    if [[ -f "$PID_FILE" ]]; then
        kill -- "-$(cat "$PID_FILE")" 2>/dev/null || true
        rm -f "$PID_FILE"
    fi
    for _ in $(seq 1 10); do
        running || { log "Stopped."; return 0; }
        sleep 1
    done
    log "Something else is still serving :$PORT." >&2
    exit 1
}

case "${1:-start}" in
    start) start ;;
    stop) stop ;;
    restart) stop && start ;;
    status) running && log "Running on :$PORT and :$TLS_PORT." || { log "Not running."; exit 1; } ;;
    *) echo "usage: $0 start|stop|restart|status" >&2; exit 2 ;;
esac
