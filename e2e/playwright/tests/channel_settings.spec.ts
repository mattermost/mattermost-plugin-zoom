// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {expect, test} from '../support/fixture';
import {setPMIPreference} from '../support/kv';
import {patchPluginSettings} from '../support/plugin';
import {openChannel, postWithText, runSlashCommand} from '../support/ui';

const disabledMessage = 'Creating Zoom meeting is disabled for this channel.';

/**
 * @objective Verify an admin can disable Zoom meetings for a channel and users are then blocked from starting one there.
 *
 * @precondition
 * A system admin with a seeded Zoom connection; the plugin opens the dialog with the admin's Zoom client.
 */
test('disables meetings for a channel from the channel settings dialog', {tag: '@zoom_plugin'}, async ({pw, zoom}) => {
    // # Create and connect a system admin
    const admin = await pw.createNewUserProfile(zoom.adminClient, {prefix: 'zoomadmin', disableTutorial: true, disableOnboarding: true});
    await zoom.adminClient.updateUserRoles(admin.id, 'system_user system_admin');
    await zoom.adminClient.addToTeam(zoom.team.id, admin.id);
    await zoom.connect(admin);
    const adminView = await openChannel(pw, admin, zoom.team.name);

    // # Disable meetings in the channel settings dialog
    await runSlashCommand(adminView.channelsPage, '/zoom channel-settings');
    const dialog = adminView.page.getByRole('dialog', {name: 'Set Channel Preference'});
    await dialog.getByLabel('Disable Zoom Meetings in this channel').check();
    await dialog.getByRole('button', {name: 'Submit'}).click();
    await expect(dialog).toBeHidden();

    // * Verify the channel is listed in the channel preferences
    await runSlashCommand(adminView.channelsPage, '/zoom channel-settings list');
    await expect(postWithText(adminView.page, 'Channel preferences').getByRole('row', {name: new RegExp(zoom.channel.id)})).toBeVisible();

    // # Run /zoom start in that channel as a connected user
    await zoom.connect();
    setPMIPreference(zoom.user.id, 'false');
    const userView = await openChannel(pw, zoom.user, zoom.team.name);
    await runSlashCommand(userView.channelsPage, '/zoom start');

    // * Verify the meeting is blocked
    await expect(postWithText(userView.page, disabledMessage)).toBeVisible();
});

/**
 * @objective Verify the plugin-wide restriction blocks meetings in public channels.
 *
 * @precondition
 * The user's Zoom connection is seeded in the KV store and RestrictMeetingCreation is on for this test.
 */
test('blocks meetings in public channels when meeting creation is restricted', {tag: '@zoom_plugin'}, async ({pw, zoom}) => {
    // # Turn on the restriction
    await zoom.connect();
    setPMIPreference(zoom.user.id, 'false');
    await patchPluginSettings(zoom.adminClient, {restrictmeetingcreation: true});
    try {
        // # Run /zoom start in a public channel
        const {channelsPage, page} = await openChannel(pw, zoom.user, zoom.team.name);
        await runSlashCommand(channelsPage, '/zoom start');

        // * Verify the meeting is blocked
        await expect(postWithText(page, disabledMessage)).toBeVisible();
    } finally {
        await patchPluginSettings(zoom.adminClient, {restrictmeetingcreation: false});
    }
});
