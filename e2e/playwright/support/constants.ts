// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

/* eslint-disable no-process-env */

import fs from 'node:fs';
import path from 'node:path';

// docker compose reads the same file, so MM_PORT/MM_IMAGE stay in sync between the stack and the tests.
const envFile = path.join(__dirname, '../.env');
if (fs.existsSync(envFile)) {
    process.loadEnvFile(envFile);
}

export const pluginId = 'zoom';
export const pluginDistDir = path.join(__dirname, '../../../dist');
export const certDir = path.join(__dirname, '../.certs');

// Must match the server's SiteURL in docker-compose.yml.
process.env.PW_BASE_URL ??= `http://127.0.0.1:${process.env.MM_PORT ?? 8065}`;
export const mattermostURL = process.env.PW_BASE_URL;

// fake-zoom resolves from the Mattermost container (extra_hosts) and from Chromium (--host-resolver-rules).
// OAuth and join links go over http; the API is https because the plugin only trusts https download URLs.
export const fakeZoomHost = 'fake-zoom';
export const fakeZoomPort = Number(process.env.FAKE_ZOOM_PORT ?? 8091);
export const fakeZoomTLSPort = Number(process.env.FAKE_ZOOM_TLS_PORT ?? 8444);
export const fakeZoomURL = `http://${fakeZoomHost}:${fakeZoomPort}`;
export const fakeZoomAPIURL = `https://${fakeZoomHost}:${fakeZoomTLSPort}/v2`;
export const fakeZoomDownloadURL = `https://${fakeZoomHost}:${fakeZoomTLSPort}`;
export const fakeZoomControlURL = `http://localhost:${fakeZoomPort}`;

export const oauthClientId = 'e2e-zoom-client-id';
export const oauthClientSecret = 'e2e-zoom-client-secret';

// Mattermost's ?secret= on the webhook URL, and Zoom's "Secret Token" used to sign the body.
export const webhookSecret = 'e2e-mattermost-webhook-secret';
export const zoomWebhookSecret = 'e2e-zoom-webhook-secret-token';

// AES-256 needs a 32-byte key.
export const encryptionKey = 'e2e-encryption-key-0123456789abc';
