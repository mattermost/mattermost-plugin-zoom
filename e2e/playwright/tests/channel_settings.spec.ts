// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {expect, test} from '../support/fixture';
import {setPMIPreference} from '../support/kv';
import {patchPluginSettings} from '../support/plugin';
import {openChannel, postWithText, runSlashCommand} from '../support/ui';

const disabledMessage = 'Creating Zoom meeting is disabled for this channel.';

test('an admin disables Zoom meetings for a channel from the channel settings dialog', async ({pw, zoom}) => {
    const admin = await pw.createNewUserProfile(zoom.adminClient, {prefix: 'zoomadmin', disableTutorial: true, disableOnboarding: true});
    await zoom.adminClient.updateUserRoles(admin.id, 'system_user system_admin');
    await zoom.adminClient.addToTeam(zoom.team.id, admin.id);

    // The plugin opens this dialog with the admin's Zoom client, so the admin must be connected.
    await zoom.connect(admin);
    const adminView = await openChannel(pw, admin, zoom.team.name);

    await runSlashCommand(adminView.channelsPage, '/zoom channel-settings');
    const dialog = adminView.page.getByRole('dialog', {name: 'Set Channel Preference'});
    await dialog.getByLabel('Disable Zoom Meetings in this channel').check();
    await dialog.getByRole('button', {name: 'Submit'}).click();
    await expect(dialog).toBeHidden();

    await runSlashCommand(adminView.channelsPage, '/zoom channel-settings list');
    await expect(postWithText(adminView.page, 'Channel preferences').getByRole('row', {name: new RegExp(zoom.channel.id)})).toBeVisible();

    await zoom.connect();
    setPMIPreference(zoom.user.id, 'false');
    const userView = await openChannel(pw, zoom.user, zoom.team.name);
    await runSlashCommand(userView.channelsPage, '/zoom start');
    await expect(postWithText(userView.page, disabledMessage)).toBeVisible();
});

test('the plugin-wide restriction blocks meetings in public channels', async ({pw, zoom}) => {
    await zoom.connect();
    setPMIPreference(zoom.user.id, 'false');
    await patchPluginSettings(zoom.adminClient, {restrictmeetingcreation: true});
    try {
        const {channelsPage, page} = await openChannel(pw, zoom.user, zoom.team.name);

        await runSlashCommand(channelsPage, '/zoom start');

        await expect(postWithText(page, disabledMessage)).toBeVisible();
    } finally {
        await patchPluginSettings(zoom.adminClient, {restrictmeetingcreation: false});
    }
});
