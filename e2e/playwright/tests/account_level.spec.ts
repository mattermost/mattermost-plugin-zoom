// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {mattermostURL, pluginId} from '../support/constants';
import {fakeZoom} from '../support/fake_zoom';
import {expect, test} from '../support/fixture';
import {deleteKey, hasKey, seedSuperUserToken, setPMIPreference, superUserTokenKey} from '../support/kv';
import {patchPluginSettings} from '../support/plugin';
import {openChannel, postWithText, runSlashCommand} from '../support/ui';

const connectURL = `${mattermostURL}/plugins/${pluginId}/oauth2/connect`;

test.describe('account-level app', () => {
    test.beforeEach(async ({zoom}) => {
        await fakeZoom.setAccountLevelApp(true);
        await patchPluginSettings(zoom.adminClient, {accountlevelapp: true});
    });

    test.afterEach(async ({zoom}) => {
        await patchPluginSettings(zoom.adminClient, {accountlevelapp: false});
        deleteKey(superUserTokenKey);
    });

    test('an admin connects once and any user on the Zoom account can start meetings', async ({pw, zoom}) => {
        const admin = await pw.createNewUserProfile(zoom.adminClient, {prefix: 'zoomadmin', disableTutorial: true, disableOnboarding: true});
        await zoom.adminClient.updateUserRoles(admin.id, 'system_user system_admin');
        await zoom.adminClient.addToTeam(zoom.team.id, admin.id);
        const adminZoom = await fakeZoom.createUser(admin.email);
        const userZoom = await fakeZoom.createUser(zoom.user.email);
        await fakeZoom.authorizeAs(adminZoom.id);

        const adminView = await openChannel(pw, admin, zoom.team.name);
        await runSlashCommand(adminView.channelsPage, '/zoom connect');
        await expect(postWithText(adminView.page, 'Click here to link your Zoom account.')).toBeVisible();
        const oauthTab = await adminView.page.context().newPage();
        await oauthTab.goto(connectURL);
        await expect(postWithText(adminView.page, 'Successfully connected to Zoom')).toBeVisible();

        expect(await fakeZoom.calls()).toEqual(['GET /oauth/authorize', 'POST /oauth/token', `GET /v2/users/${admin.email}`]);
        expect(hasKey(superUserTokenKey)).toBe(true);

        setPMIPreference(zoom.user.id, 'false');
        const userView = await openChannel(pw, zoom.user, zoom.team.name);
        await runSlashCommand(userView.channelsPage, '/zoom start');

        await expect(postWithText(userView.page, 'I have started a meeting')).toBeVisible();
        const [meeting] = await fakeZoom.meetings();
        expect(meeting.host_id).toBe(userZoom.id);
        const create = (await fakeZoom.requests()).find((r) => r.method === 'POST' && r.path.endsWith('/meetings'));
        expect(create?.path).toBe(`/v2/users/${zoom.user.email}/meetings`);
    });

    test('only system admins can connect the account-level app', async ({pw, zoom}) => {
        const {channelsPage, page} = await openChannel(pw, zoom.user, zoom.team.name);

        await runSlashCommand(channelsPage, '/zoom connect');

        await expect(postWithText(page, /Unknown Action\s*connect/)).toBeVisible();
        expect(await fakeZoom.calls()).toEqual([]);
    });

    test('a user whose email is not on the Zoom account is told why', async ({pw, zoom}) => {
        const adminZoom = await fakeZoom.createUser(`account-admin-${Date.now()}@example.com`);
        seedSuperUserToken(await fakeZoom.issueTokens(adminZoom.id));
        const {channelsPage, page} = await openChannel(pw, zoom.user, zoom.team.name);

        await runSlashCommand(channelsPage, '/zoom start');

        await expect(postWithText(page, 'We could not verify your Mattermost account in Zoom.')).toBeVisible();
        expect(await fakeZoom.meetings()).toEqual([]);
    });
});
