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
    test('answers Zoom\'s endpoint URL validation challenge', async () => {
        const res = await deliverWebhook(zoomEvents.urlValidation('plain-token-123'));

        expect(res.status).toBe(200);
        expect(await res.json()).toEqual({plainToken: 'plain-token-123', encryptedToken: sign(zoomWebhookSecret, 'plain-token-123')});
    });

    test('rejects deliveries with a bad signature, a stale timestamp, or the wrong Mattermost secret', async ({zoom}) => {
        await zoom.connect();
        const {meeting, post} = await startMeeting(zoom.userClient, zoom.user.id, zoom.channel.id);
        const event = zoomEvents.meetingEnded(meeting);

        expect((await deliverWebhook(event, {signingSecret: 'not-the-secret'})).status).toBe(401);
        expect((await deliverWebhook(event, {timestamp: Math.floor(Date.now() / 1000) - 600})).status).toBe(401);
        expect((await deliverWebhook(event, {mattermostSecret: 'not-the-secret'})).status).toBe(401);

        expect((await zoom.userClient.getPost(post.id)).props.meeting_status).toBe('STARTED');
    });

    test('meeting.ended turns the meeting post into a summary', async ({pw, zoom}) => {
        await zoom.connect();
        const {meeting, post} = await startMeeting(zoom.userClient, zoom.user.id, zoom.channel.id, 'Retro');
        const {page} = await openChannel(pw, zoom.user, zoom.team.name);

        expect((await deliverWebhook(zoomEvents.meetingStarted(meeting))).status).toBe(200);
        expect((await deliverWebhook(zoomEvents.meetingEnded(meeting))).status).toBe(200);

        const ended = postWithText(page, 'The meeting has ended.');
        await expect(ended.getByRole('heading', {name: 'Meeting Summary'})).toBeVisible();
        await expect(ended).toContainText(new RegExp(`Meeting ID :\\s*${meeting.id}`));
        expect((await zoom.userClient.getPost(post.id)).props.meeting_status).toBe('ENDED');
    });

    test('meeting.ended finds the post by meeting ID when Zoom sends a new UUID', async ({zoom}) => {
        await zoom.connect();
        const {meeting, post} = await startMeeting(zoom.userClient, zoom.user.id, zoom.channel.id);

        expect((await deliverWebhook(zoomEvents.meetingEnded(meeting, 'a-different-occurrence-uuid=='))).status).toBe(200);

        expect((await zoom.userClient.getPost(post.id)).props.meeting_status).toBe('ENDED');
    });

    test('recording.completed posts the recording link, its password, and the chat log in the meeting thread', async ({zoom}) => {
        await zoom.connect();
        const {meeting, post} = await startMeeting(zoom.userClient, zoom.user.id, zoom.channel.id);
        const chat = await fakeZoom.createDownload('10:00:01 From E2E Zoom User to Everyone: hello');
        const video = await fakeZoom.createDownload('mp4');

        const res = await deliverWebhook(zoomEvents.recordingCompleted(meeting, chat.downloadToken, [
            {file_type: 'MP4', recording_type: 'shared_screen_with_speaker_view', download_url: video.downloadURL, play_url: video.playURL},
            {file_type: 'CHAT', recording_type: 'chat_file', download_url: chat.downloadURL},
        ], 'rec-pass-123'));
        expect(res.status).toBe(200);

        const [reply] = await threadReplies(zoom.userClient, post.id);
        expect(reply.message).toContain(`[Meeting Recording](${video.playURL})`);
        expect(reply.message).toContain('**Password:** `rec-pass-123`');
        expect(reply.type).toBe('custom_zoom_chat');
        const [file] = await zoom.userClient.getFileInfosForPost(reply.id);
        expect(file.name).toBe('Chat-history.txt');

        const download = (await fakeZoom.requests()).find((r) => r.path === new URL(chat.downloadURL).pathname);
        expect(download?.headers.authorization).toBe(`Bearer ${chat.downloadToken}`);
    });

    test('recording.transcript_completed attaches the transcript to the meeting thread', async ({zoom}) => {
        await zoom.connect();
        const {meeting, post} = await startMeeting(zoom.userClient, zoom.user.id, zoom.channel.id);
        const transcript = await fakeZoom.createDownload('WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nhello');

        const res = await deliverWebhook(zoomEvents.transcriptCompleted(meeting, transcript.downloadToken, [
            {file_type: 'TRANSCRIPT', recording_type: 'audio_transcript', download_url: transcript.downloadURL},
        ]));
        expect(res.status).toBe(200);

        const [reply] = await threadReplies(zoom.userClient, post.id);
        expect(reply).toMatchObject({message: 'Here\'s the zoom meeting transcription', type: 'custom_zoom_transcript'});
        const [file] = await zoom.userClient.getFileInfosForPost(reply.id);
        expect(file.name).toBe('transcription.txt');
    });

    test('refuses to download recordings from a host that is not Zoom', async ({zoom}) => {
        await zoom.connect();
        const {meeting, post} = await startMeeting(zoom.userClient, zoom.user.id, zoom.channel.id);

        const res = await deliverWebhook(zoomEvents.recordingCompleted(meeting, 'any-token', [
            {file_type: 'CHAT', recording_type: 'chat_file', download_url: 'https://attacker.example.com/rec/download/chat'},
        ]));

        expect(res.status).toBe(500);
        expect(await threadReplies(zoom.userClient, post.id)).toEqual([]);
    });

    test('deauthorization disconnects the user and tells them', async ({zoom}) => {
        const {zoomUser} = await zoom.connect();

        expect((await deliverDeauthorization(zoomUser.id)).status).toBe(200);

        expect(readConnection(zoom.user.id)).toBeUndefined();

        // Zoom deprecated the Data Compliance API, so deauthorization must not call back into Zoom.
        expect(await fakeZoom.calls()).toEqual([]);

        const bot = await zoom.adminClient.getUserByUsername('zoom');
        const dm = await zoom.userClient.createDirectChannel([zoom.user.id, bot.id]);
        const {order, posts} = await zoom.userClient.getPosts(dm.id);
        expect(order.map((id) => posts[id].message)).toContainEqual(expect.stringContaining('We have received a deauthorization message from Zoom for your account.'));
    });
});
