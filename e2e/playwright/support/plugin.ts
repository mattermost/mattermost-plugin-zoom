// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import fs from 'node:fs/promises';
import path from 'node:path';

import type {Client4} from '@mattermost/client';

import {encryptionKey, fakeZoomAPIURL, fakeZoomURL, oauthClientId, oauthClientSecret, pluginDistDir, pluginId, webhookSecret, zoomWebhookSecret} from './constants';

export const defaultPluginSettings = {
    zoomurl: fakeZoomURL,
    zoomapiurl: fakeZoomAPIURL,
    accountlevelapp: false,
    oauthclientid: oauthClientId,
    oauthclientsecret: oauthClientSecret,
    encryptionkey: encryptionKey,
    webhooksecret: webhookSecret,
    zoomwebhooksecret: zoomWebhookSecret,
    restrictmeetingcreation: false,
    enablepostingrecordingpassword: true,
};

export async function findPluginBundle(): Promise<string> {
    const files = await fs.readdir(pluginDistDir).catch(() => [] as string[]);
    const bundles = await Promise.all(
        files.
            filter((name) => name.startsWith(`${pluginId}-`) && name.endsWith('.tar.gz')).
            map(async (name) => {
                const fullPath = path.join(pluginDistDir, name);
                return {fullPath, mtime: (await fs.stat(fullPath)).mtimeMs};
            }),
    );
    if (!bundles.length) {
        throw new Error(`No ${pluginId} bundle found in ${pluginDistDir}; run \`make dist\` first`);
    }
    return bundles.sort((a, b) => b.mtime - a.mtime)[0].fullPath;
}

// The plugin refuses to activate until the OAuth app, encryption key, and webhook secret are set.
export async function installPlugin(adminClient: Client4) {
    const bundlePath = await findPluginBundle();
    const data = await fs.readFile(bundlePath);
    const file = new File([data], path.basename(bundlePath), {type: 'application/gzip'});

    await adminClient.uploadPlugin(file as unknown as globalThis.File, true);
    await configureServer(adminClient);
    await adminClient.enablePlugin(pluginId);
    await waitForPluginRunning(adminClient);
}

export async function patchPluginSettings(adminClient: Client4, settings: Partial<typeof defaultPluginSettings>) {
    const config = await adminClient.getConfig();
    await adminClient.patchConfig({
        PluginSettings: {
            Plugins: {
                ...config.PluginSettings.Plugins,
                [pluginId]: {...config.PluginSettings.Plugins[pluginId], ...settings},
            },
        },
    });
}

async function configureServer(adminClient: Client4) {
    await adminClient.patchConfig({
        ServiceSettings: {EnableTutorial: false, EnableOnboardingFlow: false},

        // playwright-lib's random users use a short password that only passes these relaxed rules.
        PasswordSettings: {MinimumLength: 5, Lowercase: false, Number: false, Uppercase: false, Symbol: false},
    });
    await patchPluginSettings(adminClient, defaultPluginSettings);
}

export async function waitForPluginRunning(adminClient: Client4, timeoutMs = 60_000) {
    const deadline = Date.now() + timeoutMs;
    let lastState = 'unknown';
    while (Date.now() < deadline) {
        const statuses = await adminClient.getPluginStatuses();
        const status = statuses.find((s) => s.plugin_id === pluginId);
        if (status?.state === 2) {
            return;
        }
        lastState = status ? `state=${status.state}` : 'not installed';
        await new Promise((r) => setTimeout(r, 500));
    }
    throw new Error(`Plugin ${pluginId} did not reach Running: ${lastState}`);
}
