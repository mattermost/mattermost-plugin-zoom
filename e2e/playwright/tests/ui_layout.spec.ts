// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Page} from '@playwright/test';

import {expect, test} from '../support/fixture';
import {expectInside, expectNoOverlap, expectNothingClipped} from '../support/layout';
import {openChannel} from '../support/ui';
import {createMeetingPost, endMeetingPost} from '../support/zoom_posts';

// Message attachments have no accessible role or name to locate them by.
function meetingAttachment(page: Page, postId: string) {
    return page.locator(`#post_${postId}`).locator('.attachment');
}

/**
 * @objective Verify a started meeting post keeps its title, meeting ID and join button inside the post.
 *
 * @precondition
 * The meeting post is created through the API; Zoom is not involved.
 */
test('keeps title, meeting ID and join button inside a started meeting post', {tag: '@zoom_plugin'}, async ({pw, zoom}) => {
    // # Open a channel with a started meeting post
    const post = await createMeetingPost(zoom.userClient, zoom.channel.id, {topic: 'Design review'});
    const {page} = await openChannel(pw, zoom.user, zoom.team.name);
    const attachment = meetingAttachment(page, post.id);
    const join = attachment.getByRole('link', {name: 'JOIN MEETING'});
    await expect(join).toBeVisible();

    // * Verify the title and join button sit inside the post and nothing is clipped
    await expectInside(join, attachment);
    await expectInside(attachment.getByRole('heading', {name: 'Design review'}), attachment);
    await expectNothingClipped(attachment);

    // * Verify the post's accessible structure
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

/**
 * @objective Verify a long meeting topic wraps instead of overflowing the post.
 *
 * @precondition
 * The meeting post is created through the API; Zoom is not involved.
 */
test('wraps long meeting topic inside the post', {tag: '@zoom_plugin'}, async ({pw, zoom}) => {
    // # Open a channel with a meeting post that has a very long topic
    const topic = 'Quarterly cross-team planning sync for the platform reliability roadmap '.repeat(4).trim();
    const post = await createMeetingPost(zoom.userClient, zoom.channel.id, {topic});
    const {page} = await openChannel(pw, zoom.user, zoom.team.name);
    const attachment = meetingAttachment(page, post.id);
    const title = attachment.getByRole('heading', {name: topic});
    await expect(title).toBeVisible();

    // * Verify the title and join button stay inside the post
    await expectInside(title, attachment);
    await expectInside(attachment.getByRole('link', {name: 'JOIN MEETING'}), attachment);
    await expectNothingClipped(attachment);
});

/**
 * @objective Verify the recent-meeting prompt shows both buttons without overlap, including in a narrow window.
 *
 * @precondition
 * The meeting post is created through the API; Zoom is not involved.
 */
test('shows recent-meeting prompt buttons without overlap at wide and narrow widths', {tag: '@zoom_plugin'}, async ({pw, zoom}) => {
    // # Open a channel with a recent-meeting prompt
    const post = await createMeetingPost(zoom.userClient, zoom.channel.id, {status: 'RECENTLY_CREATED'});
    const {page} = await openChannel(pw, zoom.user, zoom.team.name);
    const attachment = meetingAttachment(page, post.id);
    const create = attachment.getByRole('button', {name: 'CREATE NEW MEETING'});
    const join = attachment.getByRole('link', {name: 'JOIN EXISTING MEETING'});

    for (const width of [1280, 900]) {
        // # Resize the window
        await page.setViewportSize({width, height: 900});
        await expect(join).toBeVisible();

        // * Verify the buttons don't overlap and stay inside the post
        await expectNoOverlap(create, join);
        await expectInside(create, attachment);
        await expectInside(join, attachment);
        await expectNothingClipped(attachment);
    }

    // * Verify the prompt's accessible structure
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

/**
 * @objective Verify an ended meeting summary fits the post and keeps the width of the started post.
 *
 * @precondition
 * The meeting post is created and ended through the API; Zoom is not involved.
 */
test('keeps post width when the meeting post turns into a summary', {tag: '@zoom_plugin'}, async ({pw, zoom}) => {
    // # Open a channel with a started meeting post and measure it
    const started = await createMeetingPost(zoom.userClient, zoom.channel.id);
    const {page} = await openChannel(pw, zoom.user, zoom.team.name);
    const attachment = meetingAttachment(page, started.id);
    await expect(attachment.getByRole('link', {name: 'JOIN MEETING'})).toBeVisible();
    const startedWidth = (await attachment.boundingBox())!.width;

    // # End the meeting
    await endMeetingPost(zoom.userClient, started);

    // * Verify the summary fits the post and the width is unchanged
    const summary = attachment.getByRole('heading', {name: 'Meeting Summary'});
    await expect(summary).toBeVisible();
    await expectInside(summary, attachment);
    await expectInside(attachment.getByText('Meeting Length: 30 minute(s)'), attachment);
    await expectNothingClipped(attachment);
    expect(Math.abs((await attachment.boundingBox())!.width - startedWidth)).toBeLessThanOrEqual(1);

    // * Verify the summary's accessible structure
    await expect(attachment).toMatchAriaSnapshot(`
      - paragraph: The meeting has ended.
      - heading "Zoom Meeting" [level=5]:
        - paragraph: Zoom Meeting
      - text: "/Meeting ID : \\\\d+/"
      - heading "Meeting Summary" [level=2]
      - text: "/Date: .+ Meeting Length: 30 minute\\\\(s\\\\)/"
    `);
});

/**
 * @objective Verify a meeting post in the reply thread fits the narrower right-hand panel.
 *
 * @precondition
 * The meeting post is created through the API; Zoom is not involved.
 */
test('fits meeting post inside the reply thread panel', {tag: '@zoom_plugin'}, async ({pw, zoom}) => {
    // # Open the meeting post's reply thread
    const post = await createMeetingPost(zoom.userClient, zoom.channel.id, {topic: 'Incident retro'});
    const {channelsPage} = await openChannel(pw, zoom.user, zoom.team.name);
    await (await channelsPage.centerView.getPostById(post.id)).reply();
    const rhsPost = await channelsPage.sidebarRight.getPostById(post.id);
    const attachment = rhsPost.container.locator('.attachment');
    const join = attachment.getByRole('link', {name: 'JOIN MEETING'});
    await expect(join).toBeVisible();

    // * Verify the join button stays inside the panel and the post
    await expectInside(join, channelsPage.sidebarRight.container);
    await expectInside(join, attachment);
    await expectNothingClipped(attachment);
});

/**
 * @objective Verify the app bar icon image loads and renders at icon size.
 */
test('renders app bar icon at icon size', {tag: '@zoom_plugin'}, async ({pw, zoom}) => {
    // # Open a channel
    const {page} = await openChannel(pw, zoom.user, zoom.team.name);
    const icon = page.locator('#app-bar-icon-zoom img');
    await expect(icon).toBeVisible();

    // * Verify the image loaded and is a square between 16 and 40 pixels
    await expect.poll(() => icon.evaluate((img: HTMLImageElement) => img.complete && img.naturalWidth > 0), {message: 'app-bar-icon.png should load'}).toBe(true);
    const {width, height} = (await icon.boundingBox())!;
    expect(width).toBeGreaterThanOrEqual(16);
    expect(width).toBeLessThanOrEqual(40);
    expect(Math.abs(width - height)).toBeLessThanOrEqual(1);
});

/**
 * @objective Verify started and recent-meeting posts have no axe accessibility violations.
 *
 * @precondition
 * The meeting posts are created through the API; Zoom is not involved.
 */
test('has no accessibility violations in meeting posts', {tag: '@zoom_plugin'}, async ({pw, zoom, axe}) => {
    // # Open a channel with a started meeting post and a recent-meeting prompt
    const started = await createMeetingPost(zoom.userClient, zoom.channel.id, {topic: 'Accessibility check'});
    const recent = await createMeetingPost(zoom.userClient, zoom.channel.id, {status: 'RECENTLY_CREATED'});
    const {page} = await openChannel(pw, zoom.user, zoom.team.name);
    await expect(meetingAttachment(page, recent.id).getByRole('button', {name: 'CREATE NEW MEETING'})).toBeVisible();

    // # Run axe on both posts
    const results = await axe.builder(page).include(`#post_${started.id} .attachment`).include(`#post_${recent.id} .attachment`).analyze();

    // * Verify there are no violations
    expect(results.violations.map((v) => `${v.id}: ${v.help} (${v.nodes.length})`)).toEqual([]);
});
