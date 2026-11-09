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

    /**
     * @objective Verify one admin connection lets any user on the Zoom account start meetings as themselves.
     *
     * @precondition
     * The plugin and the fake Zoom are in account-level app mode.
     */
    test('starts meetings for any account user after a single admin connect', {tag: '@zoom_plugin'}, async ({pw, zoom}) => {
        // # Create a system admin and Zoom users for the admin and the regular user
        const admin = await pw.createNewUserProfile(zoom.adminClient, {prefix: 'zoomadmin', disableTutorial: true, disableOnboarding: true});
        await zoom.adminClient.updateUserRoles(admin.id, 'system_user system_admin');
        await zoom.adminClient.addToTeam(zoom.team.id, admin.id);
        const adminZoom = await fakeZoom.createUser(admin.email);
        const userZoom = await fakeZoom.createUser(zoom.user.email);
        await fakeZoom.authorizeAs(adminZoom.id);

        // # Connect the account-level app as the admin
        const adminView = await openChannel(pw, admin, zoom.team.name);
        await runSlashCommand(adminView.channelsPage, '/zoom connect');
        await expect(postWithText(adminView.page, 'Click here to link your Zoom account.')).toBeVisible();
        const oauthTab = await adminView.page.context().newPage();
        await oauthTab.goto(connectURL);

        // * Verify the admin is connected by email and the account token is stored
        await expect(postWithText(adminView.page, 'Successfully connected to Zoom')).toBeVisible();
        expect(await fakeZoom.calls()).toEqual(['GET /oauth/authorize', 'POST /oauth/token', `GET /v2/users/${admin.email}`]);
        expect(hasKey(superUserTokenKey)).toBe(true);

        // # Run /zoom start as the regular user
        setPMIPreference(zoom.user.id, 'false');
        const userView = await openChannel(pw, zoom.user, zoom.team.name);
        await runSlashCommand(userView.channelsPage, '/zoom start');

        // * Verify the meeting is created for the regular user's Zoom account
        await expect(postWithText(userView.page, 'I have started a meeting')).toBeVisible();
        const [meeting] = await fakeZoom.meetings();
        expect(meeting.host_id).toBe(userZoom.id);
        const create = (await fakeZoom.requests()).find((r) => r.method === 'POST' && r.path.endsWith('/meetings'));
        expect(create?.path).toBe(`/v2/users/${zoom.user.email}/meetings`);
    });

    /**
     * @objective Verify a regular user cannot connect the account-level app.
     *
     * @precondition
     * The plugin and the fake Zoom are in account-level app mode.
     */
    test('rejects /zoom connect from a non-admin in account-level mode', {tag: '@zoom_plugin'}, async ({pw, zoom}) => {
        // # Run /zoom connect as a regular user
        const {channelsPage, page} = await openChannel(pw, zoom.user, zoom.team.name);
        await runSlashCommand(channelsPage, '/zoom connect');

        // * Verify the command is refused and Zoom is never called
        await expect(postWithText(page, /Unknown Action\s*connect/)).toBeVisible();
        expect(await fakeZoom.calls()).toEqual([]);
    });

    /**
     * @objective Verify a user whose email is not on the Zoom account is told why the meeting cannot start.
     *
     * @precondition
     * The plugin and the fake Zoom are in account-level app mode, with an admin token seeded in the KV store.
     */
    test('explains failure when user email is not on the Zoom account', {tag: '@zoom_plugin'}, async ({pw, zoom}) => {
        // # Seed an account token, then run /zoom start as a user Zoom does not know
        const adminZoom = await fakeZoom.createUser(`account-admin-${Date.now()}@example.com`);
        seedSuperUserToken(await fakeZoom.issueTokens(adminZoom.id));
        const {channelsPage, page} = await openChannel(pw, zoom.user, zoom.team.name);
        await runSlashCommand(channelsPage, '/zoom start');

        // * Verify the explanation and that no meeting is created
        await expect(postWithText(page, 'We could not verify your Mattermost account in Zoom.')).toBeVisible();
        expect(await fakeZoom.meetings()).toEqual([]);
    });
});
