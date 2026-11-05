// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {fakeZoomURL} from '../support/constants';
import {fakeZoom} from '../support/fake_zoom';
import {expect, test} from '../support/fixture';
import {readConnection, setPMIPreference} from '../support/kv';
import {openChannel, postWithText, runSlashCommand, zoomPosts} from '../support/ui';

const appBarIcon = '#app-bar-icon-zoom';

/**
 * @objective Verify the first /zoom start asks which meeting ID to use, then posts the new meeting.
 *
 * @precondition
 * The user's Zoom connection is seeded in the KV store and no meeting ID preference is set.
 */
test('asks for meeting ID preference on first /zoom start and starts the meeting', {tag: '@zoom_plugin'}, async ({pw, zoom}) => {
    // # Run /zoom start and choose a unique meeting ID
    await zoom.connect();
    const {channelsPage, page} = await openChannel(pw, zoom.user, zoom.team.name);
    await runSlashCommand(channelsPage, '/zoom start');
    const question = postWithText(page, 'Which meeting ID would you like to use for creating this meeting?');
    await question.getByRole('button', {name: 'USE A UNIQUE MEETING ID'}).click();

    // * Verify the preference is confirmed and the meeting post links to the created meeting
    await expect(postWithText(page, /All future meetings will use\s*unique\s*meeting ID/)).toBeVisible();
    const post = postWithText(page, 'I have started a meeting');
    await expect(post.getByRole('link', {name: 'JOIN MEETING'})).toBeVisible();
    const [meeting] = await fakeZoom.meetings();
    await expect(post.getByRole('link', {name: 'JOIN MEETING'})).toHaveAttribute('href', meeting.join_url);
    await expect(post).toContainText(new RegExp(`Meeting ID :\\s*${meeting.id}`));
});

/**
 * @objective Verify /zoom start with a topic creates an instant meeting with that topic and stores it on the post.
 *
 * @precondition
 * The user's Zoom connection is seeded in the KV store with the unique meeting ID preference.
 */
test('creates instant meeting with the topic given to /zoom start', {tag: '@zoom_plugin'}, async ({pw, zoom}) => {
    // # Run /zoom start with a topic
    await zoom.connect();
    setPMIPreference(zoom.user.id, 'false');
    const {channelsPage, page} = await openChannel(pw, zoom.user, zoom.team.name);
    await runSlashCommand(channelsPage, '/zoom start Sprint planning');

    // * Verify the post shows the topic
    const post = postWithText(page, 'I have started a meeting');
    await expect(post.getByText('Sprint planning')).toBeVisible();

    // * Verify the plugin created an instant meeting for the token owner
    const create = (await fakeZoom.requests()).find((r) => r.method === 'POST' && r.path.endsWith('/meetings'));
    expect(create).toMatchObject({path: '/v2/users/me/meetings', body: {topic: 'Sprint planning', type: 1}});

    // * Verify the post props describe the meeting
    const [meeting] = await fakeZoom.meetings();
    const [stored] = await zoomPosts(zoom.userClient, zoom.channel.id);
    expect(stored.props).toMatchObject({meeting_id: meeting.id, meeting_uuid: meeting.uuid, meeting_status: 'STARTED', meeting_topic: 'Sprint planning'});
});

/**
 * @objective Verify the personal meeting ID preference reuses the PMI instead of creating a meeting.
 *
 * @precondition
 * The user's Zoom connection is seeded in the KV store with a PMI and the PMI preference.
 */
test('reuses personal meeting ID instead of creating a meeting', {tag: '@zoom_plugin'}, async ({pw, zoom}) => {
    // # Run /zoom start with the PMI preference
    const pmi = 4155550123;
    await zoom.connect(zoom.user, {pmi});
    setPMIPreference(zoom.user.id, 'true');
    const {channelsPage, page} = await openChannel(pw, zoom.user, zoom.team.name);
    await runSlashCommand(channelsPage, '/zoom start');

    // * Verify the post joins the PMI and no meeting is created
    const post = postWithText(page, 'I have started a meeting');
    await expect(post.getByRole('link', {name: 'JOIN MEETING'})).toHaveAttribute('href', `${fakeZoomURL}/j/${pmi}`);
    expect((await fakeZoom.calls()).filter((call) => call.startsWith('POST /v2/users/'))).toEqual([]);
});

/**
 * @objective Verify the app bar button starts a meeting, opens it in a new window and posts it.
 *
 * @precondition
 * The user's Zoom connection is seeded in the KV store with the unique meeting ID preference.
 */
test('starts and opens meeting from the app bar button', {tag: '@zoom_plugin'}, async ({pw, zoom}) => {
    // # Click the Zoom app bar button
    await zoom.connect();
    setPMIPreference(zoom.user.id, 'false');
    const {page} = await openChannel(pw, zoom.user, zoom.team.name);
    const popupOpened = page.waitForEvent('popup');
    await page.locator(appBarIcon).click();
    const popup = await popupOpened;

    // * Verify the meeting opens and is posted to the channel
    const [meeting] = await fakeZoom.meetings();
    await expect(popup).toHaveURL(meeting.join_url);
    await expect(popup.getByText(`Fake Zoom meeting ${meeting.id}`)).toBeVisible();
    await expect(postWithText(page, 'I have started a meeting')).toBeVisible();
});

/**
 * @objective Verify a second start within 30 seconds offers to join the recent meeting or create a new one.
 *
 * @precondition
 * The user's Zoom connection is seeded in the KV store with the unique meeting ID preference.
 */
test('offers to join recent meeting on a second start within 30 seconds', {tag: '@zoom_plugin'}, async ({pw, zoom}) => {
    // # Start a meeting, then run /zoom start again
    await zoom.connect();
    setPMIPreference(zoom.user.id, 'false');
    const {channelsPage, page} = await openChannel(pw, zoom.user, zoom.team.name);
    await runSlashCommand(channelsPage, '/zoom start');
    await expect(postWithText(page, 'I have started a meeting')).toBeVisible();
    await runSlashCommand(channelsPage, '/zoom start');

    // * Verify the prompt links to the first meeting
    const prompt = postWithText(page, 'already created a Zoom call recently');
    const [first] = await fakeZoom.meetings();
    await expect(prompt.getByRole('link', {name: 'JOIN EXISTING MEETING'})).toHaveAttribute('href', first.join_url);

    // # Choose to create a new meeting
    await prompt.getByRole('button', {name: 'CREATE NEW MEETING'}).click();

    // * Verify a second meeting is created and posted
    await expect.poll(async () => (await zoomPosts(zoom.userClient, zoom.channel.id)).length).toBe(2);
    expect(await fakeZoom.meetings()).toHaveLength(2);
});

/**
 * @objective Verify an expired access token is refreshed and the rotated refresh token is stored.
 *
 * @precondition
 * The user's Zoom connection is seeded in the KV store with an access token that expired a minute ago.
 */
test('refreshes expired token and stores the rotated refresh token', {tag: '@zoom_plugin'}, async ({pw, zoom}) => {
    // # Run /zoom start with an expired token
    const {token} = await zoom.connect(zoom.user, {expiresInSeconds: -60});
    setPMIPreference(zoom.user.id, 'false');
    const {channelsPage, page} = await openChannel(pw, zoom.user, zoom.team.name);
    await runSlashCommand(channelsPage, '/zoom start');

    // * Verify the meeting starts after a token refresh
    await expect(postWithText(page, 'I have started a meeting')).toBeVisible();
    expect(await fakeZoom.calls()).toContain('POST /oauth/token');

    // * Verify both tokens were replaced and the new expiry is in the future
    const stored = readConnection(zoom.user.id)!.OAuthToken;
    expect(stored.refresh_token).not.toBe(token.refresh_token);
    expect(stored.access_token).not.toBe(token.access_token);
    expect(new Date(stored.expiry).getTime()).toBeGreaterThan(Date.now());
});

/**
 * @objective Verify a failed meeting creation is reported to the user and nothing is posted.
 *
 * @precondition
 * The user's Zoom connection is seeded in the KV store with the unique meeting ID preference.
 */
test('reports Zoom API failure when starting a meeting', {tag: '@zoom_plugin'}, async ({pw, zoom}) => {
    // # Make meeting creation fail, then click the app bar button
    await zoom.connect();
    setPMIPreference(zoom.user.id, 'false');
    await fakeZoom.fault('POST', '/v2/users/me/meetings', 500);
    const {page} = await openChannel(pw, zoom.user, zoom.team.name);
    await page.locator(appBarIcon).click();

    // * Verify the error is shown and no meeting post is created
    await expect(postWithText(page, 'Error occurred while starting the Zoom meeting.')).toBeVisible();
    expect(await zoomPosts(zoom.userClient, zoom.channel.id)).toEqual([]);
});

/**
 * @objective Verify a token revoked on the Zoom side is reported when the user starts a meeting.
 *
 * @precondition
 * The user's Zoom connection is seeded in the KV store, then Zoom issues new tokens so the seeded ones stop working.
 */
test('reports token revoked on the Zoom side when starting a meeting', {tag: '@zoom_plugin'}, async ({pw, zoom}) => {
    // # Revoke the stored token in Zoom, then click the app bar button
    const {zoomUser} = await zoom.connect();
    setPMIPreference(zoom.user.id, 'false');
    await fakeZoom.issueTokens(zoomUser.id);
    const {page} = await openChannel(pw, zoom.user, zoom.team.name);
    await page.locator(appBarIcon).click();

    // * Verify the error is shown and no meeting post is created
    await expect(postWithText(page, 'Error fetching user from Zoom')).toBeVisible();
    expect(await zoomPosts(zoom.userClient, zoom.channel.id)).toEqual([]);
});

/**
 * @objective Verify an unconnected user who runs /zoom start is asked to connect and Zoom is not called.
 */
test('asks unconnected user to connect when starting a meeting', {tag: '@zoom_plugin'}, async ({pw, zoom}) => {
    // # Run /zoom start without a connection
    const {channelsPage, page} = await openChannel(pw, zoom.user, zoom.team.name);
    await runSlashCommand(channelsPage, '/zoom start');

    // * Verify the connect prompt and that Zoom is never called
    await expect(postWithText(page, 'Click here to link your Zoom account.')).toBeVisible();
    expect(await fakeZoom.calls()).toEqual([]);
});
