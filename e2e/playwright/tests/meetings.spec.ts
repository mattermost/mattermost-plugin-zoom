// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {fakeZoomURL} from '../support/constants';
import {fakeZoom} from '../support/fake_zoom';
import {expect, test} from '../support/fixture';
import {readConnection, setPMIPreference} from '../support/kv';
import {openChannel, postWithText, runSlashCommand, zoomPosts} from '../support/ui';

const appBarIcon = '#app-bar-icon-zoom';

test('first /zoom start asks which meeting ID to use, then starts a meeting', async ({pw, zoom}) => {
    await zoom.connect();
    const {channelsPage, page} = await openChannel(pw, zoom.user, zoom.team.name);

    await runSlashCommand(channelsPage, '/zoom start');
    const question = postWithText(page, 'Which meeting ID would you like to use for creating this meeting?');
    await question.getByRole('button', {name: 'USE A UNIQUE MEETING ID'}).click();

    await expect(postWithText(page, /All future meetings will use\s*unique\s*meeting ID/)).toBeVisible();
    const post = postWithText(page, 'I have started a meeting');
    await expect(post.getByRole('link', {name: 'JOIN MEETING'})).toBeVisible();
    const [meeting] = await fakeZoom.meetings();
    await expect(post.getByRole('link', {name: 'JOIN MEETING'})).toHaveAttribute('href', meeting.join_url);
    await expect(post).toContainText(new RegExp(`Meeting ID :\\s*${meeting.id}`));
});

test('/zoom start with a topic creates an instant meeting with that topic', async ({pw, zoom}) => {
    await zoom.connect();
    setPMIPreference(zoom.user.id, 'false');
    const {channelsPage, page} = await openChannel(pw, zoom.user, zoom.team.name);

    await runSlashCommand(channelsPage, '/zoom start Sprint planning');

    const post = postWithText(page, 'I have started a meeting');
    await expect(post.getByText('Sprint planning')).toBeVisible();

    const create = (await fakeZoom.requests()).find((r) => r.method === 'POST' && r.path.endsWith('/meetings'));
    expect(create).toMatchObject({path: '/v2/users/me/meetings', body: {topic: 'Sprint planning', type: 1}});

    const [meeting] = await fakeZoom.meetings();
    const [stored] = await zoomPosts(zoom.userClient, zoom.channel.id);
    expect(stored.props).toMatchObject({meeting_id: meeting.id, meeting_uuid: meeting.uuid, meeting_status: 'STARTED', meeting_topic: 'Sprint planning'});
});

test('the personal meeting ID preference reuses the PMI instead of creating a meeting', async ({pw, zoom}) => {
    const pmi = 4155550123;
    await zoom.connect(zoom.user, {pmi});
    setPMIPreference(zoom.user.id, 'true');
    const {channelsPage, page} = await openChannel(pw, zoom.user, zoom.team.name);

    await runSlashCommand(channelsPage, '/zoom start');

    const post = postWithText(page, 'I have started a meeting');
    await expect(post.getByRole('link', {name: 'JOIN MEETING'})).toHaveAttribute('href', `${fakeZoomURL}/j/${pmi}`);
    expect((await fakeZoom.calls()).filter((call) => call.startsWith('POST /v2/users/'))).toEqual([]);
});

test('the app bar button starts a meeting and opens it', async ({pw, zoom}) => {
    await zoom.connect();
    setPMIPreference(zoom.user.id, 'false');
    const {page} = await openChannel(pw, zoom.user, zoom.team.name);

    const popupOpened = page.waitForEvent('popup');
    await page.locator(appBarIcon).click();
    const popup = await popupOpened;

    const [meeting] = await fakeZoom.meetings();
    await expect(popup).toHaveURL(meeting.join_url);
    await expect(popup.getByText(`Fake Zoom meeting ${meeting.id}`)).toBeVisible();
    await expect(postWithText(page, 'I have started a meeting')).toBeVisible();
});

test('a second start within 30 seconds offers to join the recent meeting', async ({pw, zoom}) => {
    await zoom.connect();
    setPMIPreference(zoom.user.id, 'false');
    const {channelsPage, page} = await openChannel(pw, zoom.user, zoom.team.name);
    await runSlashCommand(channelsPage, '/zoom start');
    await expect(postWithText(page, 'I have started a meeting')).toBeVisible();

    await runSlashCommand(channelsPage, '/zoom start');

    const prompt = postWithText(page, 'already created a Zoom call recently');
    const [first] = await fakeZoom.meetings();
    await expect(prompt.getByRole('link', {name: 'JOIN EXISTING MEETING'})).toHaveAttribute('href', first.join_url);
    await prompt.getByRole('button', {name: 'CREATE NEW MEETING'}).click();

    await expect.poll(async () => (await zoomPosts(zoom.userClient, zoom.channel.id)).length).toBe(2);
    expect(await fakeZoom.meetings()).toHaveLength(2);
});

test('an expired token is refreshed and the rotated refresh token is stored', async ({pw, zoom}) => {
    const {token} = await zoom.connect(zoom.user, {expiresInSeconds: -60});
    setPMIPreference(zoom.user.id, 'false');
    const {channelsPage, page} = await openChannel(pw, zoom.user, zoom.team.name);

    await runSlashCommand(channelsPage, '/zoom start');

    await expect(postWithText(page, 'I have started a meeting')).toBeVisible();
    expect(await fakeZoom.calls()).toContain('POST /oauth/token');
    const stored = readConnection(zoom.user.id)!.OAuthToken;
    expect(stored.refresh_token).not.toBe(token.refresh_token);
    expect(stored.access_token).not.toBe(token.access_token);
    expect(new Date(stored.expiry).getTime()).toBeGreaterThan(Date.now());
});

test('a Zoom API failure is reported to the user', async ({pw, zoom}) => {
    await zoom.connect();
    setPMIPreference(zoom.user.id, 'false');
    await fakeZoom.fault('POST', '/v2/users/me/meetings', 500);
    const {page} = await openChannel(pw, zoom.user, zoom.team.name);

    await page.locator(appBarIcon).click();

    await expect(postWithText(page, 'Error occurred while starting the Zoom meeting.')).toBeVisible();
    expect(await zoomPosts(zoom.userClient, zoom.channel.id)).toEqual([]);
});

test('an unconnected user is asked to connect when starting a meeting', async ({pw, zoom}) => {
    const {channelsPage, page} = await openChannel(pw, zoom.user, zoom.team.name);

    await runSlashCommand(channelsPage, '/zoom start');

    await expect(postWithText(page, 'Click here to link your Zoom account.')).toBeVisible();
    expect(await fakeZoom.calls()).toEqual([]);
});
