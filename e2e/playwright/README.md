# Zoom plugin Playwright tests

Hermetic E2E suite: Mattermost runs in Docker, Zoom is replaced by `fake-zoom/` (started by `global_setup.ts`).
No Zoom account or secrets are needed, locally or in CI.

```sh
make e2e-env-up     # creates fake-zoom's TLS certificate, starts Postgres + Mattermost on :8065
make e2e            # builds the plugin (make dist), installs it, runs the suite
make e2e-env-down
```

The suite uses the normal `make dist` bundle. `global_setup.ts` points the plugin's Zoom URL and Zoom API URL settings
at fake-zoom, so no special build is needed.

Useful overrides, set in the shell or in `e2e/playwright/.env` (copy `.env.example`; docker compose reads it too):

| Variable | Default | Purpose |
|---|---|---|
| `MM_IMAGE` | `mattermostdevelopment/mattermost-enterprise-edition:master` | Server image (runs as amd64, emulated on Apple Silicon) |
| `MM_PORT` | `8065` | Host port; must be the same for `e2e-env-up` and `e2e` |
| `PW_BASE_URL` | `http://127.0.0.1:${MM_PORT}` | Mattermost URL the tests use; leave unset unless the server runs elsewhere |
| `FAKE_ZOOM_PORT` | `8091` | fake-zoom's http port: OAuth, join links and the test control API |
| `FAKE_ZOOM_TLS_PORT` | `8444` | fake-zoom's https port: REST API and recording downloads |
| `PW_BROWSER_PORT` | `3333` | Port of the dockerized browser used by the visual tests |
| `PW_HEADLESS` | `true` | `false` to watch the browser (`npm run test:headed`) |

`npm run test:visual` sets its own `MM_IMAGE`, `MM_PORT` (8075) and `PW_BROWSER_PORT` (3334); override the ports with
`VISUAL_MM_PORT` and `VISUAL_BROWSER_PORT`.

## How Zoom is faked

- fake-zoom serves Zoom's OAuth endpoints (`zoom.us`) and REST API (`api.zoom.us/v2`). The API and recording downloads
  are on https, because the plugin only downloads recordings from https Zoom hosts. `scripts/fake_zoom_cert.sh`
  creates a self-signed certificate that docker compose mounts into Mattermost's trusted roots.
- Mattermost resolves `fake-zoom` via `extra_hosts` and Chromium via `--host-resolver-rules`, so OAuth redirects and
  join links work in the browser.
- `fake-zoom/server.ts` implements the endpoints the plugin calls. Unhandled routes return 404 and are logged as
  `[fake-zoom] unhandled …`; add a route there when a new flow needs one. It also models Zoom rules the plugin
  depends on, such as user-level tokens only addressing `me` and a new token pair revoking the old one.
- Tests drive it through `support/fake_zoom.ts`: `reset()`, `calls()`/`requests()` (journal of every call the plugin
  made), `fault(method, path, status)` for error paths, and helpers to create users, meetings, tokens and downloads.
- Inbound webhooks are signed with the Zoom secret token in `support/zoom_webhook.ts`; those tests are tagged
  `@synthetic-inbound`.
- Only `connect.spec.ts` and `account_level.spec.ts` drive the OAuth flow. Other specs connect users with
  `zoom.connect()`, which writes the plugin's connection record into Postgres (`support/kv.ts`). Keep it in sync with
  `server/store.go` and `server/cipher.go`.

## Keeping the fake honest

- Every API request and response fake-zoom handles is validated against Zoom's published OpenAPI spec, and every
  synthetic webhook against Zoom's event schemas. The `zoom` fixture fails the test on any violation, including routes
  and events not listed in `fake-zoom/contract/operations.json`.
- New route or event: add it to `operations.json`, run `npm run contract:update`, commit `fake-zoom/contract/specs/`.
- Nightly, `npm run contract:check` fails if Zoom changed those operations upstream.

## Visual tests

`tests/visual.spec.ts` (`@visual`) screenshots the plugin's meeting posts and app bar icon and compares them with the
PNGs in `tests/__screenshots__/`. `npm run test:visual` starts a separate stack with a pinned server image plus a
dockerized browser, so baselines match on any machine and in CI (a non-blocking job). After an intended UI change, run
`npm run test:visual-update-snapshots`, look at the new PNGs, and commit them.

## Writing tests

Tests follow the [Mattermost Playwright guidelines](https://github.com/mattermost/mattermost/blob/master/e2e-tests/playwright/CLAUDE.md):
a JSDoc `@objective` (and `@precondition` when setup is unusual), an action-oriented title, the `@zoom_plugin` tag,
and `// #` action / `// *` verification comments. Prefer `getByRole`/`getByText` locators over CSS.

## Gotchas

- The plugin calls its own SiteURL, so Mattermost listens on `MM_PORT` inside the container too, and everything uses
  `127.0.0.1`.
- Don't use `pw.initSetup()`: it resets server config and drops the plugin's settings. Use the `zoom` fixture instead.
