// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Page} from '@playwright/test';

import {expect, test} from '../support/fixture';
import {expectInside, expectNoOverlap, expectNothingClipped} from '../support/layout';
import {openChannel} from '../support/ui';
import {createMeetingPost, endMeetingPost} from '../support/zoom_posts';

function meetingAttachment(page: Page, postId: string) {
    return page.locator(`#post_${postId}`).locator('.attachment');
}

test('a started meeting post keeps its title, ID and join button inside the post', async ({pw, zoom}) => {
    const post = await createMeetingPost(zoom.userClient, zoom.channel.id, {topic: 'Design review'});
    const {page} = await openChannel(pw, zoom.user, zoom.team.name);
    const attachment = meetingAttachment(page, post.id);
    const join = attachment.getByRole('link', {name: 'JOIN MEETING'});

    await expect(join).toBeVisible();
    await expectInside(join, attachment);
    await expectInside(attachment.getByRole('heading', {name: 'Design review'}), attachment);
    await expectNothingClipped(attachment);
    await expect(attachment).toMatchAriaSnapshot(`
      - paragraph: I have started a meeting
      - heading "Design review" [level=5]:
        - paragraph: Design review
      - text: "Meeting ID :"
      - link /\\d+/:
        - /url: http://fake-zoom:8091/j/81234567890
      - link "icon video small JOIN MEETING":
        - /url: http://fake-zoom:8091/j/81234567890
        - img "icon video small"
        - text: ""
    `);
});

test('a long meeting topic wraps instead of overflowing the post', async ({pw, zoom}) => {
    const topic = 'Quarterly cross-team planning sync for the platform reliability roadmap '.repeat(4).trim();
    const post = await createMeetingPost(zoom.userClient, zoom.channel.id, {topic});
    const {page} = await openChannel(pw, zoom.user, zoom.team.name);
    const attachment = meetingAttachment(page, post.id);
    const title = attachment.getByRole('heading', {name: topic});

    await expect(title).toBeVisible();
    await expectInside(title, attachment);
    await expectInside(attachment.getByRole('link', {name: 'JOIN MEETING'}), attachment);
    await expectNothingClipped(attachment);
});

test('the recent-meeting prompt shows both buttons without overlapping, even in a narrow window', async ({pw, zoom}) => {
    const post = await createMeetingPost(zoom.userClient, zoom.channel.id, {status: 'RECENTLY_CREATED'});
    const {page} = await openChannel(pw, zoom.user, zoom.team.name);
    const attachment = meetingAttachment(page, post.id);
    const create = attachment.getByRole('button', {name: 'CREATE NEW MEETING'});
    const join = attachment.getByRole('link', {name: 'JOIN EXISTING MEETING'});

    for (const width of [1280, 900]) {
        await page.setViewportSize({width, height: 900});
        await expect(join).toBeVisible();
        await expectNoOverlap(create, join);
        await expectInside(create, attachment);
        await expectInside(join, attachment);
        await expectNothingClipped(attachment);
    }
    await expect(attachment).toMatchAriaSnapshot(`
      - paragraph: zoom-e2e already created a Zoom call recently
      - heading "Zoom Meeting" [level=5]:
        - paragraph: Zoom Meeting
      - text: What do you want to do?
      - button "CREATE NEW MEETING"
      - link "icon video small JOIN EXISTING MEETING":
        - /url: http://fake-zoom:8091/j/81234567890
        - img "icon video small"
        - text: ""
    `);
});

test('an ended meeting summary fits the post and keeps the post width', async ({pw, zoom}) => {
    const started = await createMeetingPost(zoom.userClient, zoom.channel.id);
    const {page} = await openChannel(pw, zoom.user, zoom.team.name);
    const attachment = meetingAttachment(page, started.id);
    await expect(attachment.getByRole('link', {name: 'JOIN MEETING'})).toBeVisible();
    const startedWidth = (await attachment.boundingBox())!.width;

    await endMeetingPost(zoom.userClient, started);

    const summary = attachment.getByRole('heading', {name: 'Meeting Summary'});
    await expect(summary).toBeVisible();
    await expectInside(summary, attachment);
    await expectInside(attachment.getByText('Meeting Length: 30 minute(s)'), attachment);
    await expectNothingClipped(attachment);
    expect(Math.abs((await attachment.boundingBox())!.width - startedWidth)).toBeLessThanOrEqual(1);
    await expect(attachment).toMatchAriaSnapshot(`
      - paragraph: The meeting has ended.
      - heading "Zoom Meeting" [level=5]:
        - paragraph: Zoom Meeting
      - text: "/Meeting ID : \\\\d+/"
      - heading "Meeting Summary" [level=2]
      - text: "/Date: .+ Meeting Length: 30 minute\\\\(s\\\\)/"
    `);
});

test('a meeting post in the reply thread fits the narrower right-hand panel', async ({pw, zoom}) => {
    const post = await createMeetingPost(zoom.userClient, zoom.channel.id, {topic: 'Incident retro'});
    const {channelsPage} = await openChannel(pw, zoom.user, zoom.team.name);
    await (await channelsPage.centerView.getPostById(post.id)).reply();

    const rhsPost = await channelsPage.sidebarRight.getPostById(post.id);
    const attachment = rhsPost.container.locator('.attachment');
    const join = attachment.getByRole('link', {name: 'JOIN MEETING'});
    await expect(join).toBeVisible();
    await expectInside(join, channelsPage.sidebarRight.container);
    await expectInside(join, attachment);
    await expectNothingClipped(attachment);
});

test('the app bar icon loads and renders at icon size', async ({pw, zoom}) => {
    const {page} = await openChannel(pw, zoom.user, zoom.team.name);
    const icon = page.locator('#app-bar-icon-zoom img');

    await expect(icon).toBeVisible();
    expect(await icon.evaluate((img: HTMLImageElement) => img.complete && img.naturalWidth > 0), 'app-bar-icon.png should load').toBe(true);
    const {width, height} = (await icon.boundingBox())!;
    expect(width).toBeGreaterThanOrEqual(16);
    expect(width).toBeLessThanOrEqual(40);
    expect(Math.abs(width - height)).toBeLessThanOrEqual(1);
});

test('the meeting post has no accessibility violations', async ({pw, zoom, axe}) => {
    const started = await createMeetingPost(zoom.userClient, zoom.channel.id, {topic: 'Accessibility check'});
    const recent = await createMeetingPost(zoom.userClient, zoom.channel.id, {status: 'RECENTLY_CREATED'});
    const {page} = await openChannel(pw, zoom.user, zoom.team.name);
    await expect(meetingAttachment(page, recent.id).getByRole('button', {name: 'CREATE NEW MEETING'})).toBeVisible();

    const results = await axe.builder(page).include(`#post_${started.id} .attachment`).include(`#post_${recent.id} .attachment`).analyze();

    expect(results.violations.map((v) => `${v.id}: ${v.help} (${v.nodes.length})`)).toEqual([]);
});
