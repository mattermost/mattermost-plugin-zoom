// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {mattermostURL, pluginId} from '../support/constants';
import {fakeZoom} from '../support/fake_zoom';
import {expect, test} from '../support/fixture';
import {readConnection} from '../support/kv';
import {openChannel, postWithText, runSlashCommand} from '../support/ui';

const connectURL = `${mattermostURL}/plugins/${pluginId}/oauth2/connect`;

/**
 * @objective Verify a user connects through Zoom's OAuth flow and the plugin stores the Zoom account.
 */
test('connects Zoom account through OAuth and reports already connected afterwards', {tag: '@zoom_plugin'}, async ({pw, zoom}) => {
    // # Make the fake Zoom sign in as a new Zoom user
    const zoomUser = await fakeZoom.createUser(`oauth-${zoom.user.username}@example.com`);
    await fakeZoom.authorizeAs(zoomUser.id);
    const {channelsPage, page} = await openChannel(pw, zoom.user, zoom.team.name);

    // # Run /zoom connect
    await runSlashCommand(channelsPage, '/zoom connect');

    // * Verify the reply links to the plugin's connect endpoint
    const prompt = postWithText(page, 'Click here to link your Zoom account.');
    await expect(prompt.getByRole('link', {name: 'Click here to link your Zoom account.'})).toHaveAttribute('href', connectURL);

    // # Complete the OAuth flow in a second tab; the callback page closes itself
    const oauthTab = await page.context().newPage();
    await oauthTab.goto(connectURL);

    // * Verify the success reply, the Zoom calls made and the stored connection
    await expect(postWithText(page, 'Successfully connected to Zoom')).toBeVisible();
    expect(await fakeZoom.calls()).toEqual(['GET /oauth/authorize', 'POST /oauth/token', 'GET /v2/users/me']);
    expect(readConnection(zoom.user.id)).toMatchObject({ZoomID: zoomUser.id, ZoomEmail: zoomUser.email, UserID: zoom.user.id});

    // # Run /zoom connect again
    await runSlashCommand(channelsPage, '/zoom connect');

    // * Verify the user is told they are already connected
    await expect(postWithText(page, 'Already connected')).toBeVisible();
});

/**
 * @objective Verify the OAuth callback rejects a state the plugin never issued and does not exchange the code.
 */
test('rejects OAuth callback with forged state without exchanging the code', {tag: '@zoom_plugin'}, async ({pw, zoom}) => {
    // # Start the connect flow so the user has a real pending state
    const {channelsPage, page} = await openChannel(pw, zoom.user, zoom.team.name);
    await runSlashCommand(channelsPage, '/zoom connect');
    await expect(postWithText(page, 'Click here to link your Zoom account.')).toBeVisible();

    // # Call the callback with a forged state
    const callback = await page.request.get(`${mattermostURL}/plugins/${pluginId}/oauth2/complete?code=attacker-code&state=forged-state`);

    // * Verify the callback is rejected, Zoom is never called and nothing is stored
    expect(callback.status()).toBe(401);
    expect(await callback.text()).toContain('OAuth user state mismatch');
    expect(await fakeZoom.calls()).toEqual([]);
    expect(readConnection(zoom.user.id)).toBeUndefined();
});

/**
 * @objective Verify /zoom disconnect removes the stored connection.
 *
 * @precondition
 * The user's Zoom connection is seeded in the KV store.
 */
test('disconnects Zoom account with /zoom disconnect', {tag: '@zoom_plugin'}, async ({pw, zoom}) => {
    // # Run /zoom disconnect as a connected user
    await zoom.connect();
    const {channelsPage, page} = await openChannel(pw, zoom.user, zoom.team.name);
    await runSlashCommand(channelsPage, '/zoom disconnect');

    // * Verify the reply and that the connection is gone
    await expect(postWithText(page, 'User disconnected from Zoom.')).toBeVisible();
    expect(readConnection(zoom.user.id)).toBeUndefined();
});

/**
 * @objective Verify the plugin recognizes a connection written straight to the KV store, as the other specs rely on.
 *
 * @precondition
 * The user's Zoom connection is seeded in the KV store.
 */
test('recognizes KV-seeded connection without an OAuth round-trip', {tag: '@zoom_plugin'}, async ({pw, zoom}) => {
    // # Run /zoom connect as a KV-seeded user
    await zoom.connect();
    const {channelsPage, page} = await openChannel(pw, zoom.user, zoom.team.name);
    await runSlashCommand(channelsPage, '/zoom connect');

    // * Verify the user is already connected and Zoom is never called
    await expect(postWithText(page, 'Already connected')).toBeVisible();
    expect(await fakeZoom.calls()).toEqual([]);
});

/**
 * @objective Verify a failed Zoom user lookup during OAuth shows an error and stores nothing.
 */
test('shows connect error when Zoom rejects the user lookup', {tag: '@zoom_plugin'}, async ({pw, zoom}) => {
    // # Make the Zoom user lookup fail, then start the connect flow
    await fakeZoom.fault('GET', '/v2/users/me', 500);
    const {channelsPage, page} = await openChannel(pw, zoom.user, zoom.team.name);
    await runSlashCommand(channelsPage, '/zoom connect');
    await expect(postWithText(page, 'Click here to link your Zoom account.')).toBeVisible();

    // # Complete the OAuth flow in a second tab
    const oauthTab = await page.context().newPage();
    await oauthTab.goto(connectURL);

    // * Verify the error page and that nothing is stored
    await expect(oauthTab.getByText('Could not complete the connection: Error fetching user from Zoom')).toBeVisible();
    expect(readConnection(zoom.user.id)).toBeUndefined();
});
