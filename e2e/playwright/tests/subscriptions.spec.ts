// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {fakeZoomURL} from '../support/constants';
import {fakeZoom} from '../support/fake_zoom';
import {expect, test} from '../support/fixture';
import {openChannel, postWithText, runSlashCommand, zoomPosts} from '../support/ui';
import {deliverWebhook, zoomEvents} from '../support/zoom_webhook';

const personalMeetingType = 4;

test.describe('meeting subscriptions', {tag: '@synthetic-inbound'}, () => {
    /**
     * @objective Verify a host can subscribe a channel to a meeting, Zoom events post there, and unsubscribing stops them.
     *
     * @precondition
     * The user's Zoom connection is seeded in the KV store and the user hosts a scheduled meeting.
     */
    test('posts meeting events to subscribed channel until it unsubscribes', {tag: '@zoom_plugin'}, async ({pw, zoom}) => {
        // # Subscribe the channel to a meeting the user hosts
        const {zoomUser} = await zoom.connect();
        const meeting = await fakeZoom.createMeeting(zoomUser.id, 'Weekly sync');
        const {channelsPage, page} = await openChannel(pw, zoom.user, zoom.team.name);
        await runSlashCommand(channelsPage, `/zoom subscription add ${meeting.id}`);

        // * Verify the subscription is confirmed and listed
        await expect(postWithText(page, 'Channel subscribed to meeting.')).toBeVisible();
        await runSlashCommand(channelsPage, '/zoom subscription list');
        await expect(postWithText(page, 'Meeting Subscriptions').getByRole('row', {name: `${meeting.id} ~${zoom.channel.display_name}`})).toBeVisible();

        // # Deliver meeting.started
        expect((await deliverWebhook(zoomEvents.meetingStarted(meeting))).status).toBe(200);

        // * Verify the started post with the topic and join link
        const started = postWithText(page, 'zoom has started a meeting');
        await expect(started.getByRole('heading', {name: 'Weekly sync'})).toBeVisible();
        await expect(started.getByRole('link', {name: 'JOIN MEETING'})).toHaveAttribute('href', new RegExp(`^${fakeZoomURL}/j/${meeting.id}`));

        // # Deliver meeting.ended
        expect((await deliverWebhook(zoomEvents.meetingEnded(meeting))).status).toBe(200);

        // * Verify the post turns into a summary
        await expect(postWithText(page, 'zoom has ended the meeting').getByRole('heading', {name: 'Meeting Summary'})).toBeVisible();

        // # Unsubscribe, then deliver the next occurrence's meeting.started
        await runSlashCommand(channelsPage, `/zoom subscription remove ${meeting.id}`);
        await expect(postWithText(page, 'Channel unsubscribed from meeting.')).toBeVisible();
        expect((await deliverWebhook(zoomEvents.meetingStarted(meeting, 'next-occurrence-uuid=='))).status).toBe(200);

        // * Verify no new meeting post appears
        expect(await zoomPosts(zoom.userClient, zoom.channel.id)).toHaveLength(1);
    });

    /**
     * @objective Verify a user cannot subscribe a channel to a meeting hosted by someone else.
     *
     * @precondition
     * The user's Zoom connection is seeded in the KV store and another Zoom user hosts the meeting.
     */
    test('rejects subscription to a meeting hosted by another user', {tag: '@zoom_plugin'}, async ({pw, zoom}) => {
        // # Try to subscribe to another host's meeting
        await zoom.connect();
        const otherHost = await fakeZoom.createUser(`other-host-${Date.now()}@example.com`);
        const meeting = await fakeZoom.createMeeting(otherHost.id, 'Not mine');
        const {channelsPage, page} = await openChannel(pw, zoom.user, zoom.team.name);
        await runSlashCommand(channelsPage, `/zoom subscription add ${meeting.id}`);

        // * Verify the subscription is refused
        await expect(postWithText(page, 'You can only subscribe to meetings you host.')).toBeVisible();
    });

    /**
     * @objective Verify personal and unknown meetings cannot be subscribed to.
     *
     * @precondition
     * The user's Zoom connection is seeded in the KV store and the user has a personal meeting room.
     */
    test('rejects subscription to personal and unknown meetings', {tag: '@zoom_plugin'}, async ({pw, zoom}) => {
        // # Try to subscribe to the user's personal meeting
        const {zoomUser} = await zoom.connect();
        const personal = await fakeZoom.createMeeting(zoomUser.id, 'My room', personalMeetingType);
        const {channelsPage, page} = await openChannel(pw, zoom.user, zoom.team.name);
        await runSlashCommand(channelsPage, `/zoom subscription add ${personal.id}`);

        // * Verify personal meetings are refused
        await expect(postWithText(page, 'Cannot subscribe to personal meeting')).toBeVisible();

        // # Try to subscribe to a meeting Zoom does not know
        await runSlashCommand(channelsPage, '/zoom subscription add 98765432109');

        // * Verify unknown meetings are refused
        await expect(postWithText(page, 'Cannot subscribe to meeting: meeting not found')).toBeVisible();
    });
});
