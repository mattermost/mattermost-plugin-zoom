// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {fakeZoomURL} from '../support/constants';
import {fakeZoom} from '../support/fake_zoom';
import {expect, test} from '../support/fixture';
import {openChannel, postWithText, runSlashCommand, zoomPosts} from '../support/ui';
import {deliverWebhook, zoomEvents} from '../support/zoom_webhook';

const personalMeetingType = 4;

test.describe('meeting subscriptions', {tag: '@synthetic-inbound'}, () => {
    test('a host subscribes a channel and Zoom events post there until they unsubscribe', async ({pw, zoom}) => {
        const {zoomUser} = await zoom.connect();
        const meeting = await fakeZoom.createMeeting(zoomUser.id, 'Weekly sync');
        const {channelsPage, page} = await openChannel(pw, zoom.user, zoom.team.name);

        await runSlashCommand(channelsPage, `/zoom subscription add ${meeting.id}`);
        await expect(postWithText(page, 'Channel subscribed to meeting.')).toBeVisible();

        await runSlashCommand(channelsPage, '/zoom subscription list');
        await expect(postWithText(page, 'Meeting Subscriptions').getByRole('row', {name: `${meeting.id} ~${zoom.channel.display_name}`})).toBeVisible();

        expect((await deliverWebhook(zoomEvents.meetingStarted(meeting))).status).toBe(200);
        const started = postWithText(page, 'zoom has started a meeting');
        await expect(started.getByRole('heading', {name: 'Weekly sync'})).toBeVisible();
        await expect(started.getByRole('link', {name: 'JOIN MEETING'})).toHaveAttribute('href', new RegExp(`^${fakeZoomURL}/j/${meeting.id}`));

        expect((await deliverWebhook(zoomEvents.meetingEnded(meeting))).status).toBe(200);
        await expect(postWithText(page, 'zoom has ended the meeting').getByRole('heading', {name: 'Meeting Summary'})).toBeVisible();

        await runSlashCommand(channelsPage, `/zoom subscription remove ${meeting.id}`);
        await expect(postWithText(page, 'Channel unsubscribed from meeting.')).toBeVisible();

        expect((await deliverWebhook(zoomEvents.meetingStarted(meeting, 'next-occurrence-uuid=='))).status).toBe(200);
        expect(await zoomPosts(zoom.userClient, zoom.channel.id)).toHaveLength(1);
    });

    test('a user cannot subscribe to a meeting someone else hosts', async ({pw, zoom}) => {
        await zoom.connect();
        const otherHost = await fakeZoom.createUser(`other-host-${Date.now()}@example.com`);
        const meeting = await fakeZoom.createMeeting(otherHost.id, 'Not mine');
        const {channelsPage, page} = await openChannel(pw, zoom.user, zoom.team.name);

        await runSlashCommand(channelsPage, `/zoom subscription add ${meeting.id}`);

        await expect(postWithText(page, 'You can only subscribe to meetings you host.')).toBeVisible();
    });

    test('personal and unknown meetings cannot be subscribed to', async ({pw, zoom}) => {
        const {zoomUser} = await zoom.connect();
        const personal = await fakeZoom.createMeeting(zoomUser.id, 'My room', personalMeetingType);
        const {channelsPage, page} = await openChannel(pw, zoom.user, zoom.team.name);

        await runSlashCommand(channelsPage, `/zoom subscription add ${personal.id}`);
        await expect(postWithText(page, 'Cannot subscribe to personal meeting')).toBeVisible();

        await runSlashCommand(channelsPage, '/zoom subscription add 98765432109');
        await expect(postWithText(page, 'Cannot subscribe to meeting: meeting not found')).toBeVisible();
    });
});
