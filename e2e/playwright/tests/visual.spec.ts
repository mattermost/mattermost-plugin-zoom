// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {PlaywrightExtended} from '@mattermost/playwright-lib';
import type {Locator} from '@playwright/test';

import {fakeZoom} from '../support/fake_zoom';
import {expect, test, type ZoomFixture} from '../support/fixture';
import {startMeeting} from '../support/meetings';
import {openChannel, postWithText, runSlashCommand, threadReplies} from '../support/ui';
import {createMeetingPost, type MeetingPostOptions} from '../support/zoom_posts';
import {deliverWebhook, zoomEvents} from '../support/zoom_webhook';

test.describe('visual', {tag: '@visual'}, () => {
    // eslint-disable-next-line no-process-env
    test.skip(!process.env.PW_VISUAL_BROWSER_WS, 'baselines only match the dockerized browser (npm run test:visual)');

    // Core UI overlapping the post (avatars, status, timestamps) changes between runs.
    async function settle(pw: PlaywrightExtended, target: Locator) {
        const page = target.page();
        await expect(page.locator('.LoadingScreen')).toBeHidden();
        await pw.hideDynamicChannelsContent(page);
        await page.mouse.move(0, 0);
        await page.evaluate(() => document.fonts.ready);

        // The bottom-anchored post list leaves posts at a fractional y that depends on server state (a fresh
        // server renders a shorter channel intro), which moves text by a pixel row. Snap the target to whole pixels.
        await target.evaluate((el: HTMLElement) => {
            const {x, y} = el.getBoundingClientRect();
            el.style.transform = `translate(${Math.round(x) - x}px, ${Math.round(y) - y}px)`;
        });
    }

    async function meetingPost(pw: PlaywrightExtended, zoom: ZoomFixture, options: MeetingPostOptions) {
        const post = await createMeetingPost(zoom.userClient, zoom.channel.id, options);
        const {page, channelsPage} = await openChannel(pw, zoom.user, zoom.team.name);
        const attachment = page.locator(`#post_${post.id}`).locator('.attachment');
        await expect(attachment).toBeVisible();
        await settle(pw, attachment);
        return {page, channelsPage, post, attachment};
    }

    async function commandReply(pw: PlaywrightExtended, zoom: ZoomFixture, command: string, text: string, selector = '.post__body') {
        const {page, channelsPage} = await openChannel(pw, zoom.user, zoom.team.name);
        await runSlashCommand(channelsPage, command);
        const target = postWithText(page, text).locator(selector);
        await expect(target).toBeVisible();
        await settle(pw, target);
        return target;
    }

    async function threadReply(pw: PlaywrightExtended, zoom: ZoomFixture, rootId: string, readyText: string) {
        const [reply] = await threadReplies(zoom.userClient, rootId);
        const {channelsPage} = await openChannel(pw, zoom.user, zoom.team.name);
        await (await channelsPage.centerView.getPostById(rootId)).reply();
        const body = (await channelsPage.sidebarRight.getPostById(reply.id)).body;
        await expect(body.getByText(readyText)).toBeVisible();
        await settle(pw, body);
        return body;
    }

    /**
     * @objective Verify the started meeting post renders like the baseline.
     *
     * @precondition
     * Runs in the pinned dockerized browser and server (npm run test:visual).
     */
    test('matches baseline for started meeting post', {tag: '@zoom_plugin'}, async ({pw, zoom}) => {
        // # Open a channel with a started meeting post
        const {attachment} = await meetingPost(pw, zoom, {topic: 'Design review'});

        // * Verify the post matches the baseline
        await expect(attachment).toHaveScreenshot('started.png');
    });

    /**
     * @objective Verify the personal meeting ID post renders like the baseline.
     *
     * @precondition
     * Runs in the pinned dockerized browser and server (npm run test:visual).
     */
    test('matches baseline for personal meeting ID post', {tag: '@zoom_plugin'}, async ({pw, zoom}) => {
        // # Open a channel with a personal meeting ID post
        const {attachment} = await meetingPost(pw, zoom, {personal: true, meetingId: 5551234567});

        // * Verify the post matches the baseline
        await expect(attachment).toHaveScreenshot('personal.png');
    });

    /**
     * @objective Verify a meeting post with a long topic renders like the baseline.
     *
     * @precondition
     * Runs in the pinned dockerized browser and server (npm run test:visual).
     */
    test('matches baseline for long topic post', {tag: '@zoom_plugin'}, async ({pw, zoom}) => {
        // # Open a channel with a long-topic meeting post
        const topic = 'Quarterly cross-team planning sync for the platform reliability roadmap '.repeat(4).trim();
        const {attachment} = await meetingPost(pw, zoom, {topic});

        // * Verify the post matches the baseline
        await expect(attachment).toHaveScreenshot('long-topic.png');
    });

    /**
     * @objective Verify the recent-meeting prompt renders like the baseline.
     *
     * @precondition
     * Runs in the pinned dockerized browser and server (npm run test:visual).
     */
    test('matches baseline for recent meeting prompt', {tag: '@zoom_plugin'}, async ({pw, zoom}) => {
        // # Open a channel with a recent-meeting prompt
        const {attachment} = await meetingPost(pw, zoom, {status: 'RECENTLY_CREATED'});

        // * Verify the prompt matches the baseline
        await expect(attachment).toHaveScreenshot('recently-created.png');
    });

    /**
     * @objective Verify the ended meeting summary renders like the baseline.
     *
     * @precondition
     * Runs in the pinned dockerized browser and server (npm run test:visual).
     */
    test('matches baseline for ended meeting summary', {tag: '@zoom_plugin'}, async ({pw, zoom}) => {
        // # Open a channel with an ended meeting post
        const {attachment} = await meetingPost(pw, zoom, {status: 'ENDED'});

        // # Replace the date, which is the post's creation time, so the line's width stays stable
        await attachment.getByText(/^Date:/).evaluate((el) => {
            el.textContent = 'Date: Jan 1 at 12:00 PM';
        });

        // * Verify the summary matches the baseline
        await expect(attachment).toHaveScreenshot('ended.png');
    });

    /**
     * @objective Verify a meeting post in the reply thread renders like the baseline.
     *
     * @precondition
     * Runs in the pinned dockerized browser and server (npm run test:visual).
     */
    test('matches baseline for meeting post in reply thread', {tag: '@zoom_plugin'}, async ({pw, zoom}) => {
        // # Open the meeting post's reply thread
        const {channelsPage, post} = await meetingPost(pw, zoom, {topic: 'Incident retro'});
        await (await channelsPage.centerView.getPostById(post.id)).reply();
        const rhsPost = await channelsPage.sidebarRight.getPostById(post.id);
        const attachment = rhsPost.container.locator('.attachment');
        await expect(attachment).toBeVisible();
        await settle(pw, attachment);

        // * Verify the thread post matches the baseline
        await expect(attachment).toHaveScreenshot('reply-thread.png');
    });

    /**
     * @objective Verify the app bar icon renders like the baseline.
     *
     * @precondition
     * Runs in the pinned dockerized browser and server (npm run test:visual).
     */
    test('matches baseline for app bar icon', {tag: '@zoom_plugin'}, async ({pw, zoom}) => {
        // # Open a channel
        const {page} = await openChannel(pw, zoom.user, zoom.team.name);
        const icon = page.locator('#app-bar-icon-zoom');
        await expect(icon.locator('img')).toBeVisible();
        await settle(pw, icon);

        // * Verify the icon matches the baseline
        await expect(icon).toHaveScreenshot('app-bar-icon.png');
    });

    /**
     * @objective Verify the ended personal meeting ID summary renders like the baseline.
     *
     * @precondition
     * Runs in the pinned dockerized browser and server (npm run test:visual).
     */
    test('matches baseline for ended personal meeting summary', {tag: '@zoom_plugin'}, async ({pw, zoom}) => {
        // # Open a channel with an ended personal meeting ID post
        const {attachment} = await meetingPost(pw, zoom, {status: 'ENDED', personal: true, meetingId: 5551234567});

        // # Replace the date, which is the post's creation time, so the line's width stays stable
        await attachment.getByText(/^Date:/).evaluate((el) => {
            el.textContent = 'Date: Jan 1 at 12:00 PM';
        });

        // * Verify the summary matches the baseline
        await expect(attachment).toHaveScreenshot('ended-personal.png');
    });

    /**
     * @objective Verify the recent-meeting prompt renders like the baseline in a narrow window.
     *
     * @precondition
     * Runs in the pinned dockerized browser and server (npm run test:visual).
     */
    test('matches baseline for recent meeting prompt at narrow width', {tag: '@zoom_plugin'}, async ({pw, zoom}) => {
        // # Open a channel with a recent-meeting prompt in a narrow window
        const post = await createMeetingPost(zoom.userClient, zoom.channel.id, {status: 'RECENTLY_CREATED'});
        const {page} = await openChannel(pw, zoom.user, zoom.team.name);
        await page.setViewportSize({width: 480, height: 900});
        const attachment = page.locator(`#post_${post.id}`).locator('.attachment');
        await expect(attachment.getByRole('button', {name: 'CREATE NEW MEETING'})).toBeVisible();
        await settle(pw, attachment);

        // * Verify the prompt matches the baseline
        await expect(attachment).toHaveScreenshot('recently-created-narrow.png');
    });

    /**
     * @objective Verify the meeting ID choice prompt renders like the baseline.
     *
     * @precondition
     * Runs in the pinned dockerized browser and server (npm run test:visual).
     */
    test('matches baseline for meeting ID choice prompt', {tag: '@zoom_plugin'}, async ({pw, zoom}) => {
        // # Run /zoom start as a connected user without a meeting ID preference
        await zoom.connect();
        const attachment = await commandReply(pw, zoom, '/zoom start', 'Which meeting ID would you like to use for creating this meeting?', '.mm-blocks');

        // * Verify the prompt matches the baseline
        await expect(attachment).toHaveScreenshot('meeting-id-prompt.png');
    });

    /**
     * @objective Verify the personal meeting ID setting form renders like the baseline.
     *
     * @precondition
     * Runs in the pinned dockerized browser and server (npm run test:visual).
     */
    test('matches baseline for personal meeting ID setting form', {tag: '@zoom_plugin'}, async ({pw, zoom}) => {
        // # Run /zoom settings as a connected user
        await zoom.connect();
        const attachment = await commandReply(pw, zoom, '/zoom settings', 'Do you want to use your Personal Meeting ID when starting a meeting?', '.mm-blocks');

        // * Verify the form matches the baseline
        await expect(attachment).toHaveScreenshot('pmi-setting.png');
    });

    /**
     * @objective Verify the connect prompt for an unconnected user renders like the baseline.
     *
     * @precondition
     * Runs in the pinned dockerized browser and server (npm run test:visual).
     */
    test('matches baseline for connect prompt', {tag: '@zoom_plugin'}, async ({pw, zoom}) => {
        // # Run /zoom start without a connection
        const body = await commandReply(pw, zoom, '/zoom start', 'Click here to link your Zoom account.');

        // * Verify the prompt matches the baseline
        await expect(body).toHaveScreenshot('connect-prompt.png');
    });

    /**
     * @objective Verify the meeting subscription list renders like the baseline.
     *
     * @precondition
     * Runs in the pinned dockerized browser and server (npm run test:visual).
     */
    test('matches baseline for subscription list', {tag: '@zoom_plugin'}, async ({pw, zoom}) => {
        // # Subscribe the channel to a meeting the user hosts, then list subscriptions
        const {zoomUser} = await zoom.connect();
        const meeting = await fakeZoom.createMeeting(zoomUser.id, 'Weekly sync', 2, 81111111111);
        const {channelsPage, page} = await openChannel(pw, zoom.user, zoom.team.name);
        await runSlashCommand(channelsPage, `/zoom subscription add ${meeting.id}`);
        await expect(postWithText(page, 'Channel subscribed to meeting.')).toBeVisible();
        await runSlashCommand(channelsPage, '/zoom subscription list');
        const body = postWithText(page, 'Meeting Subscriptions').locator('.post__body');
        await expect(body.getByRole('row', {name: `${meeting.id} ~${zoom.channel.display_name}`})).toBeVisible();
        await settle(pw, body);

        // * Verify the list matches the baseline
        await expect(body).toHaveScreenshot('subscription-list.png');
    });

    /**
     * @objective Verify the bot's started meeting post for a subscribed channel renders like the baseline.
     *
     * @precondition
     * Runs in the pinned dockerized browser and server (npm run test:visual).
     */
    test('matches baseline for subscribed meeting started post', {tag: '@zoom_plugin'}, async ({pw, zoom}) => {
        // # Subscribe the channel to a meeting and deliver meeting.started
        const {zoomUser} = await zoom.connect();
        const meeting = await fakeZoom.createMeeting(zoomUser.id, 'Weekly sync', 2, 82222222222);
        const {channelsPage, page} = await openChannel(pw, zoom.user, zoom.team.name);
        await runSlashCommand(channelsPage, `/zoom subscription add ${meeting.id}`);
        await expect(postWithText(page, 'Channel subscribed to meeting.')).toBeVisible();
        expect((await deliverWebhook(zoomEvents.meetingStarted(meeting))).status).toBe(200);
        const attachment = postWithText(page, 'zoom has started a meeting').locator('.attachment');
        await expect(attachment.getByRole('link', {name: 'JOIN MEETING'})).toBeVisible();
        await settle(pw, attachment);

        // * Verify the post matches the baseline
        await expect(attachment).toHaveScreenshot('subscription-started.png');
    });

    /**
     * @objective Verify the recording reply in the meeting thread renders like the baseline.
     *
     * @precondition
     * Runs in the pinned dockerized browser and server (npm run test:visual).
     */
    test('matches baseline for recording reply', {tag: '@zoom_plugin'}, async ({pw, zoom}) => {
        // # Start a meeting and deliver recording.completed with a video
        await zoom.connect();
        const {meeting, post} = await startMeeting(zoom.userClient, zoom.user.id, zoom.channel.id);
        const video = await fakeZoom.createDownload('mp4');
        expect((await deliverWebhook(zoomEvents.recordingCompleted(meeting, video.downloadToken, [
            {file_type: 'MP4', recording_type: 'shared_screen_with_speaker_view', download_url: video.downloadURL, play_url: video.playURL},
        ], 'rec-pass-123'))).status).toBe(200);

        // # Open the meeting thread
        const body = await threadReply(pw, zoom, post.id, 'Meeting Recording');

        // * Verify the reply matches the baseline
        await expect(body).toHaveScreenshot('recording-reply.png');
    });

    /**
     * @objective Verify the chat history reply in the meeting thread renders like the baseline.
     *
     * @precondition
     * Runs in the pinned dockerized browser and server (npm run test:visual).
     */
    test('matches baseline for chat history reply', {tag: '@zoom_plugin'}, async ({pw, zoom}) => {
        // # Start a meeting and deliver recording.completed with a chat file
        await zoom.connect();
        const {meeting, post} = await startMeeting(zoom.userClient, zoom.user.id, zoom.channel.id);
        const chat = await fakeZoom.createDownload('10:00:01 From E2E Zoom User to Everyone: hello');
        expect((await deliverWebhook(zoomEvents.recordingCompleted(meeting, chat.downloadToken, [
            {file_type: 'CHAT', recording_type: 'chat_file', download_url: chat.downloadURL},
        ]))).status).toBe(200);

        // # Open the meeting thread
        const body = await threadReply(pw, zoom, post.id, 'Chat-history.txt');

        // * Verify the reply matches the baseline
        await expect(body).toHaveScreenshot('chat-history-reply.png');
    });

    /**
     * @objective Verify the transcript reply in the meeting thread renders like the baseline.
     *
     * @precondition
     * Runs in the pinned dockerized browser and server (npm run test:visual).
     */
    test('matches baseline for transcript reply', {tag: '@zoom_plugin'}, async ({pw, zoom}) => {
        // # Start a meeting and deliver recording.transcript_completed
        await zoom.connect();
        const {meeting, post} = await startMeeting(zoom.userClient, zoom.user.id, zoom.channel.id);
        const transcript = await fakeZoom.createDownload('WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nhello');
        expect((await deliverWebhook(zoomEvents.transcriptCompleted(meeting, transcript.downloadToken, [
            {file_type: 'TRANSCRIPT', recording_type: 'audio_transcript', download_url: transcript.downloadURL},
        ]))).status).toBe(200);

        // # Open the meeting thread
        const body = await threadReply(pw, zoom, post.id, 'transcription.txt');

        // * Verify the reply matches the baseline
        await expect(body).toHaveScreenshot('transcript-reply.png');
    });

    /**
     * @objective Verify the channel preference dialog renders like the baseline.
     *
     * @precondition
     * Runs in the pinned dockerized browser and server (npm run test:visual).
     */
    test('matches baseline for channel preference dialog', {tag: '@zoom_plugin'}, async ({pw, zoom}) => {
        // # Run /zoom channel-settings as a connected system admin
        const admin = await pw.createNewUserProfile(zoom.adminClient, {prefix: 'zoomadmin', disableTutorial: true, disableOnboarding: true});
        await zoom.adminClient.updateUserRoles(admin.id, 'system_user system_admin');
        await zoom.adminClient.addToTeam(zoom.team.id, admin.id);
        await zoom.connect(admin);
        const {channelsPage, page} = await openChannel(pw, admin, zoom.team.name);
        await runSlashCommand(channelsPage, '/zoom channel-settings');
        const dialog = page.getByRole('dialog', {name: 'Set Channel Preference'}).locator('.modal-content');
        await expect(dialog.getByLabel('Disable Zoom Meetings in this channel')).toBeVisible();

        // The modal slides in, so its final position is only known once the transition ends.
        await page.locator('.modal.in .modal-dialog').evaluate((el) => Promise.all(el.getAnimations().map((a) => a.finished)));
        await settle(pw, dialog);

        // * Verify the dialog matches the baseline
        await expect(dialog).toHaveScreenshot('channel-preference-dialog.png');
    });
});
