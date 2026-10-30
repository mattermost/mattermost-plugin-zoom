// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {mattermostURL, pluginId} from '../support/constants';
import {fakeZoom} from '../support/fake_zoom';
import {expect, test} from '../support/fixture';
import {readConnection} from '../support/kv';
import {openChannel, postWithText, runSlashCommand} from '../support/ui';

const connectURL = `${mattermostURL}/plugins/${pluginId}/oauth2/connect`;

test('user connects through the Zoom OAuth round-trip', async ({pw, zoom}) => {
    const zoomUser = await fakeZoom.createUser(`oauth-${zoom.user.username}@example.com`);
    await fakeZoom.authorizeAs(zoomUser.id);
    const {channelsPage, page} = await openChannel(pw, zoom.user, zoom.team.name);

    await runSlashCommand(channelsPage, '/zoom connect');
    const prompt = postWithText(page, 'Click here to link your Zoom account.');
    await expect(prompt.getByRole('link', {name: 'Click here to link your Zoom account.'})).toHaveAttribute('href', connectURL);

    // The callback page closes itself, so the flow runs in a second tab while the first one shows the ephemeral reply.
    const oauthTab = await page.context().newPage();
    await oauthTab.goto(connectURL);
    await expect(postWithText(page, 'Successfully connected to Zoom')).toBeVisible();

    expect(await fakeZoom.calls()).toEqual(['GET /oauth/authorize', 'POST /oauth/token', 'GET /v2/users/me']);
    expect(readConnection(zoom.user.id)).toMatchObject({ZoomID: zoomUser.id, ZoomEmail: zoomUser.email, UserID: zoom.user.id});

    await runSlashCommand(channelsPage, '/zoom connect');
    await expect(postWithText(page, 'Already connected')).toBeVisible();
});

test('the OAuth callback rejects a forged state without exchanging the code', async ({pw, zoom}) => {
    const {channelsPage, page} = await openChannel(pw, zoom.user, zoom.team.name);
    await runSlashCommand(channelsPage, '/zoom connect');
    await expect(postWithText(page, 'Click here to link your Zoom account.')).toBeVisible();

    const callback = await page.request.get(`${mattermostURL}/plugins/${pluginId}/oauth2/complete?code=attacker-code&state=forged-state`);

    expect(callback.status()).toBe(401);
    expect(await callback.text()).toContain('OAuth user state mismatch');
    expect(await fakeZoom.calls()).toEqual([]);
    expect(readConnection(zoom.user.id)).toBeUndefined();
});

test('user disconnects', async ({pw, zoom}) => {
    await zoom.connect();
    const {channelsPage, page} = await openChannel(pw, zoom.user, zoom.team.name);

    await runSlashCommand(channelsPage, '/zoom disconnect');

    await expect(postWithText(page, 'User disconnected from Zoom.')).toBeVisible();
    expect(readConnection(zoom.user.id)).toBeUndefined();
});

test('a KV-seeded connection is recognized without an OAuth round-trip', async ({pw, zoom}) => {
    await zoom.connect();
    const {channelsPage, page} = await openChannel(pw, zoom.user, zoom.team.name);

    await runSlashCommand(channelsPage, '/zoom connect');

    await expect(postWithText(page, 'Already connected')).toBeVisible();
    expect(await fakeZoom.calls()).toEqual([]);
});

test('connect fails gracefully when Zoom rejects the user lookup', async ({pw, zoom}) => {
    await fakeZoom.fault('GET', '/v2/users/me', 500);
    const {channelsPage, page} = await openChannel(pw, zoom.user, zoom.team.name);
    await runSlashCommand(channelsPage, '/zoom connect');
    await expect(postWithText(page, 'Click here to link your Zoom account.')).toBeVisible();

    const oauthTab = await page.context().newPage();
    await oauthTab.goto(connectURL);

    await expect(oauthTab.getByText('Could not complete the connection: Error fetching user from Zoom')).toBeVisible();
    expect(readConnection(zoom.user.id)).toBeUndefined();
});
