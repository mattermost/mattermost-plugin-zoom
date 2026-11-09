// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {zoomWebhookSecret} from '../support/constants';
import {fakeZoom} from '../support/fake_zoom';
import {expect, test} from '../support/fixture';
import {readConnection} from '../support/kv';
import {startMeeting} from '../support/meetings';
import {openChannel, postWithText, threadReplies} from '../support/ui';
import {deliverDeauthorization, deliverWebhook, sign, zoomEvents} from '../support/zoom_webhook';

test.describe('Zoom webhooks', {tag: '@synthetic-inbound'}, () => {
    /**
     * @objective Verify the webhook endpoint answers Zoom's URL validation challenge with the signed token.
     */
    test('answers Zoom endpoint URL validation challenge', {tag: '@zoom_plugin'}, async () => {
        // # Deliver a URL validation event
        const res = await deliverWebhook(zoomEvents.urlValidation('plain-token-123'));

        // * Verify the plain and encrypted tokens are returned
        expect(res.status).toBe(200);
        expect(await res.json()).toEqual({plainToken: 'plain-token-123', encryptedToken: sign(zoomWebhookSecret, 'plain-token-123')});
    });

    /**
     * @objective Verify deliveries with a bad signature, a stale timestamp or the wrong Mattermost secret are rejected.
     *
     * @precondition
     * The user's Zoom connection is seeded in the KV store.
     */
    test('rejects webhook with bad signature, stale timestamp or wrong Mattermost secret', {tag: '@zoom_plugin'}, async ({zoom}) => {
        // # Start a meeting and build its meeting.ended event
        await zoom.connect();
        const {meeting, post} = await startMeeting(zoom.userClient, zoom.user.id, zoom.channel.id);
        const event = zoomEvents.meetingEnded(meeting);

        // * Verify each tampered delivery is rejected
        expect((await deliverWebhook(event, {signingSecret: 'not-the-secret'})).status).toBe(401);
        expect((await deliverWebhook(event, {timestamp: Math.floor(Date.now() / 1000) - 600})).status).toBe(401);
        expect((await deliverWebhook(event, {mattermostSecret: 'not-the-secret'})).status).toBe(401);

        // * Verify the meeting post is unchanged
        expect((await zoom.userClient.getPost(post.id)).props.meeting_status).toBe('STARTED');
    });

    /**
     * @objective Verify meeting.ended turns the meeting post into a summary.
     *
     * @precondition
     * The user's Zoom connection is seeded in the KV store.
     */
    test('turns meeting post into a summary on meeting.ended', {tag: '@zoom_plugin'}, async ({pw, zoom}) => {
        // # Start a meeting, open the channel and deliver meeting.started then meeting.ended
        await zoom.connect();
        const {meeting, post} = await startMeeting(zoom.userClient, zoom.user.id, zoom.channel.id, 'Retro');
        const {page} = await openChannel(pw, zoom.user, zoom.team.name);
        expect((await deliverWebhook(zoomEvents.meetingStarted(meeting))).status).toBe(200);
        expect((await deliverWebhook(zoomEvents.meetingEnded(meeting))).status).toBe(200);

        // * Verify the post shows the summary and is marked ended
        const ended = postWithText(page, 'The meeting has ended.');
        await expect(ended.getByRole('heading', {name: 'Meeting Summary'})).toBeVisible();
        await expect(ended).toContainText(new RegExp(`Meeting ID :\\s*${meeting.id}`));
        expect((await zoom.userClient.getPost(post.id)).props.meeting_status).toBe('ENDED');
    });

    /**
     * @objective Verify meeting.ended still finds the post by meeting ID when Zoom sends a new occurrence UUID.
     *
     * @precondition
     * The user's Zoom connection is seeded in the KV store.
     */
    test('ends meeting post found by meeting ID when Zoom sends a new UUID', {tag: '@zoom_plugin'}, async ({zoom}) => {
        // # Start a meeting and deliver meeting.ended with a different UUID
        await zoom.connect();
        const {meeting, post} = await startMeeting(zoom.userClient, zoom.user.id, zoom.channel.id);
        expect((await deliverWebhook(zoomEvents.meetingEnded(meeting, 'a-different-occurrence-uuid=='))).status).toBe(200);

        // * Verify the post is marked ended
        expect((await zoom.userClient.getPost(post.id)).props.meeting_status).toBe('ENDED');
    });

    /**
     * @objective Verify recording.completed posts the recording link, its password and the chat log in the meeting thread.
     *
     * @precondition
     * The user's Zoom connection is seeded in the KV store.
     */
    test('posts recording link, password and chat log in thread on recording.completed', {tag: '@zoom_plugin'}, async ({zoom}) => {
        // # Start a meeting and deliver recording.completed with a video and a chat file
        await zoom.connect();
        const {meeting, post} = await startMeeting(zoom.userClient, zoom.user.id, zoom.channel.id);
        const chat = await fakeZoom.createDownload('10:00:01 From E2E Zoom User to Everyone: hello');
        const video = await fakeZoom.createDownload('mp4');
        const res = await deliverWebhook(zoomEvents.recordingCompleted(meeting, chat.downloadToken, [
            {file_type: 'MP4', recording_type: 'shared_screen_with_speaker_view', download_url: video.downloadURL, play_url: video.playURL},
            {file_type: 'CHAT', recording_type: 'chat_file', download_url: chat.downloadURL},
        ], 'rec-pass-123'));
        expect(res.status).toBe(200);

        // * Verify the thread reply has the recording link, password and chat file
        const [reply] = await threadReplies(zoom.userClient, post.id);
        expect(reply.message).toContain(`[Meeting Recording](${video.playURL})`);
        expect(reply.message).toContain('**Password:** `rec-pass-123`');
        expect(reply.type).toBe('custom_zoom_chat');
        const [file] = await zoom.userClient.getFileInfosForPost(reply.id);
        expect(file.name).toBe('Chat-history.txt');

        // * Verify the chat file was downloaded with the event's download token
        const download = (await fakeZoom.requests()).find((r) => r.path === new URL(chat.downloadURL).pathname);
        expect(download?.headers.authorization).toBe(`Bearer ${chat.downloadToken}`);
    });

    /**
     * @objective Verify recording.transcript_completed attaches the transcript to the meeting thread.
     *
     * @precondition
     * The user's Zoom connection is seeded in the KV store.
     */
    test('attaches transcript to meeting thread on recording.transcript_completed', {tag: '@zoom_plugin'}, async ({zoom}) => {
        // # Start a meeting and deliver recording.transcript_completed
        await zoom.connect();
        const {meeting, post} = await startMeeting(zoom.userClient, zoom.user.id, zoom.channel.id);
        const transcript = await fakeZoom.createDownload('WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nhello');
        const res = await deliverWebhook(zoomEvents.transcriptCompleted(meeting, transcript.downloadToken, [
            {file_type: 'TRANSCRIPT', recording_type: 'audio_transcript', download_url: transcript.downloadURL},
        ]));
        expect(res.status).toBe(200);

        // * Verify the thread reply carries the transcript file
        const [reply] = await threadReplies(zoom.userClient, post.id);
        expect(reply).toMatchObject({message: 'Here\'s the zoom meeting transcription', type: 'custom_zoom_transcript'});
        const [file] = await zoom.userClient.getFileInfosForPost(reply.id);
        expect(file.name).toBe('transcription.txt');
    });

    /**
     * @objective Verify the plugin refuses to download recording files from a host that is not Zoom.
     *
     * @precondition
     * The user's Zoom connection is seeded in the KV store.
     */
    test('refuses to download recordings from a non-Zoom host', {tag: '@zoom_plugin'}, async ({zoom}) => {
        // # Start a meeting and deliver recording.completed pointing at another host
        await zoom.connect();
        const {meeting, post} = await startMeeting(zoom.userClient, zoom.user.id, zoom.channel.id);
        const res = await deliverWebhook(zoomEvents.recordingCompleted(meeting, 'any-token', [
            {file_type: 'CHAT', recording_type: 'chat_file', download_url: 'https://attacker.example.com/rec/download/chat'},
        ]));

        // * Verify the delivery fails and nothing is posted in the thread
        expect(res.status).toBe(500);
        expect(await threadReplies(zoom.userClient, post.id)).toEqual([]);
    });

    /**
     * @objective Verify deauthorization disconnects the user, DMs them and does not call back into Zoom.
     *
     * @precondition
     * The user's Zoom connection is seeded in the KV store.
     */
    test('disconnects user and sends DM on deauthorization without calling Zoom', {tag: '@zoom_plugin'}, async ({zoom}) => {
        // # Deliver a deauthorization event for the connected Zoom user
        const {zoomUser} = await zoom.connect();
        expect((await deliverDeauthorization(zoomUser.id)).status).toBe(200);

        // * Verify the connection is removed and Zoom's retired Data Compliance API is not called
        expect(readConnection(zoom.user.id)).toBeUndefined();
        expect(await fakeZoom.calls()).toEqual([]);

        // * Verify the bot DMs the user about the deauthorization
        const bot = await zoom.adminClient.getUserByUsername('zoom');
        const dm = await zoom.userClient.createDirectChannel([zoom.user.id, bot.id]);
        const {order, posts} = await zoom.userClient.getPosts(dm.id);
        expect(order.map((id) => posts[id].message)).toContainEqual(expect.stringContaining('We have received a deauthorization message from Zoom for your account.'));
    });
});
