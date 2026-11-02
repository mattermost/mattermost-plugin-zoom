// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {PlaywrightExtended} from '@mattermost/playwright-lib';
import type {Locator} from '@playwright/test';

import {expect, test, type ZoomFixture} from '../support/fixture';
import {openChannel} from '../support/ui';
import {createMeetingPost, type MeetingPostOptions} from '../support/zoom_posts';

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
});
