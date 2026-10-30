// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Client4} from '@mattermost/client';
import type {Channel} from '@mattermost/types/channels';
import type {Team} from '@mattermost/types/teams';
import type {UserProfile} from '@mattermost/types/users';
import {expect, test as base} from '@mattermost/playwright-lib';

import type {IssuedToken, ZoomUser} from '../fake-zoom/state';

import {fakeZoom} from './fake_zoom';
import {seedConnection} from './kv';

export {expect};

type ZoomConnection = {zoomUser: ZoomUser; token: IssuedToken};

export type ZoomFixture = {
    adminClient: Client4;
    user: UserProfile;
    userClient: Client4;
    team: Team;
    channel: Channel;
    connect: (mattermostUser?: UserProfile, options?: {pmi?: number; expiresInSeconds?: number}) => Promise<ZoomConnection>;
};

export type FailureScreenshotOptions = {failureScreenshots: boolean};

// pw.initSetup() is deliberately avoided: it resets server config, which drops the plugin's settings and PluginStates.
export const test = base.extend<{zoom: ZoomFixture; failureScreenshot: void}, FailureScreenshotOptions>({
    failureScreenshots: [false, {option: true, scope: 'worker'}],

    // Playwright's own failure screenshot runs after pw closes the context and only sees a blank page.
    failureScreenshot: [async ({pw, failureScreenshots}, use, testInfo) => {
        await use();
        if (!failureScreenshots || testInfo.status === testInfo.expectedStatus) {
            return;
        }
        for (const [i, page] of (pw.testBrowser.context?.pages() ?? []).entries()) {
            const body = await page.screenshot({fullPage: true}).catch(() => undefined);
            if (body) {
                await testInfo.attach(`failure-page-${i}`, {body, contentType: 'image/png'});
            }
        }
    }, {auto: true}],

    zoom: async ({pw}, use) => {
        await fakeZoom.reset();

        const {adminClient} = await pw.getAdminClient();
        const team = await pw.createNewTeam(adminClient, {name: 'zoom', displayName: 'Zoom', type: 'O', unique: true});
        const user = await pw.createNewUserProfile(adminClient, {prefix: 'zoom', disableTutorial: true, disableOnboarding: true});
        await adminClient.addToTeam(team.id, user.id);
        const {client: userClient} = await pw.makeClient(user);
        if (!userClient) {
            throw new Error(`Failed to log in ${user.username}`);
        }
        const channel = await adminClient.getChannelByName(team.id, 'town-square');

        const connect: ZoomFixture['connect'] = async (mattermostUser = user, options = {}) => {
            const zoomUser = await fakeZoom.createUser(`zoom-${mattermostUser.username}@example.com`, options.pmi);
            const token = await fakeZoom.issueTokens(zoomUser.id, options.expiresInSeconds);
            seedConnection(mattermostUser.id, zoomUser, token);
            return {zoomUser, token};
        };

        await use({adminClient, user, userClient, team, channel, connect});

        expect(await fakeZoom.contractViolations(), 'fake-zoom API traffic must match Zoom\'s OpenAPI spec').toEqual([]);
    },
});
