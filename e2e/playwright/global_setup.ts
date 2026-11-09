// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {baseGlobalSetup} from '@mattermost/playwright-lib';

import {startFakeZoom} from './fake-zoom/server';
import {loginAdmin} from './support/admin';
import {certDir, fakeZoomAPIURL, fakeZoomDownloadURL, fakeZoomPort, fakeZoomTLSPort, fakeZoomURL, mattermostURL, oauthClientId, oauthClientSecret, pluginId} from './support/constants';
import {installPlugin} from './support/plugin';

export default async function globalSetup() {
    await baseGlobalSetup();

    const fake = await startFakeZoom({
        port: fakeZoomPort,
        tlsPort: fakeZoomTLSPort,
        certDir,
        zoomURL: fakeZoomURL,
        apiURL: fakeZoomAPIURL,
        downloadURL: fakeZoomDownloadURL,
        clientId: oauthClientId,
        clientSecret: oauthClientSecret,
        redirectURI: `${mattermostURL}/plugins/${pluginId}/oauth2/complete`,
    });

    // Reinstalling restarts the plugin process, so it picks up a regenerated fake-zoom certificate.
    const adminClient = await loginAdmin();
    await installPlugin(adminClient);

    return fake.close;
}
