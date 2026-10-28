// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {PlaywrightExtended} from '@mattermost/playwright-lib';
import type {Locator, Page} from '@playwright/test';

import {expect, test, type ZoomFixture} from '../support/fixture';
import {openChannel} from '../support/ui';
import {createMeetingPost, type MeetingPostOptions} from '../support/zoom_posts';

test.describe('@visual', () => {
    // eslint-disable-next-line no-process-env
    test.skip(!process.env.PW_VISUAL_BROWSER_WS, 'baselines only match the dockerized browser (npm run test:visual)');

    // Core UI overlapping the post (avatars, status, timestamps) changes between runs.
    async function settle(pw: PlaywrightExtended, page: Page) {
        await expect(page.locator('.LoadingScreen')).toBeHidden();
        await pw.hideDynamicChannelsContent(page);
        await page.mouse.move(0, 0);
        await page.evaluate(() => document.fonts.ready);
    }

    async function meetingPost(pw: PlaywrightExtended, zoom: ZoomFixture, options: MeetingPostOptions) {
        const post = await createMeetingPost(zoom.userClient, zoom.channel.id, options);
        const {page, channelsPage} = await openChannel(pw, zoom.user, zoom.team.name);
        const attachment = page.locator(`#post_${post.id}`).locator('.attachment');
        await expect(attachment).toBeVisible();
        await settle(pw, page);
        return {page, channelsPage, post, attachment};
    }

    const dateLine = (attachment: Locator) => attachment.getByText(/^Date:/);

    test('started meeting post', async ({pw, zoom}) => {
        const {attachment} = await meetingPost(pw, zoom, {topic: 'Design review'});
        await expect(attachment).toHaveScreenshot('started.png');
    });

    test('personal meeting ID post', async ({pw, zoom}) => {
        const {attachment} = await meetingPost(pw, zoom, {personal: true, meetingId: 5551234567});
        await expect(attachment).toHaveScreenshot('personal.png');
    });

    test('long topic post', async ({pw, zoom}) => {
        const topic = 'Quarterly cross-team planning sync for the platform reliability roadmap '.repeat(4).trim();
        const {attachment} = await meetingPost(pw, zoom, {topic});
        await expect(attachment).toHaveScreenshot('long-topic.png');
    });

    test('recent meeting prompt', async ({pw, zoom}) => {
        const {attachment} = await meetingPost(pw, zoom, {status: 'RECENTLY_CREATED'});
        await expect(attachment).toHaveScreenshot('recently-created.png');
    });

    test('ended meeting summary', async ({pw, zoom}) => {
        const {attachment} = await meetingPost(pw, zoom, {status: 'ENDED'});
        await expect(attachment).toHaveScreenshot('ended.png', {mask: [dateLine(attachment)]});
    });

    test('meeting post in the reply thread', async ({pw, zoom}) => {
        const {page, channelsPage, post} = await meetingPost(pw, zoom, {topic: 'Incident retro'});
        await (await channelsPage.centerView.getPostById(post.id)).reply();
        const rhsPost = await channelsPage.sidebarRight.getPostById(post.id);
        const attachment = rhsPost.container.locator('.attachment');
        await expect(attachment).toBeVisible();
        await settle(pw, page);
        await expect(attachment).toHaveScreenshot('reply-thread.png');
    });

    test('app bar icon', async ({pw, zoom}) => {
        const {page} = await openChannel(pw, zoom.user, zoom.team.name);
        const icon = page.locator('#app-bar-icon-zoom');
        await expect(icon.locator('img')).toBeVisible();
        await settle(pw, page);
        await expect(icon).toHaveScreenshot('app-bar-icon.png');
    });
});
