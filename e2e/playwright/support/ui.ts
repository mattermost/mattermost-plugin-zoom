// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Client4} from '@mattermost/client';
import type {Post} from '@mattermost/types/posts';
import type {UserProfile} from '@mattermost/types/users';
import {expect, type ChannelsPage, type PlaywrightExtended} from '@mattermost/playwright-lib';
import type {Page} from '@playwright/test';

export async function openChannel(pw: PlaywrightExtended, user: UserProfile, teamName: string, channelName = 'town-square') {
    const {channelsPage, page} = await pw.testBrowser.login(user);
    await channelsPage.goto(teamName, channelName);
    await channelsPage.toBeVisible();
    return {channelsPage, page};
}

export async function runSlashCommand(channelsPage: ChannelsPage, command: string) {
    const postCreate = channelsPage.centerView.postCreate;
    await postCreate.input.fill(command);

    // Enter would pick the highlighted slash-command autocomplete suggestion instead of sending.
    await postCreate.sendMessageButton.click();
    await expect(postCreate.input).toHaveValue('');
}

export function postWithText(page: Page, text: string | RegExp) {
    return page.locator('.post').filter({hasText: text}).last();
}

export async function zoomPosts(client: Client4, channelId: string): Promise<Post[]> {
    const {order, posts} = await client.getPosts(channelId);
    return order.map((id) => posts[id]).filter((post) => (post.type as string) === 'custom_zoom');
}

export async function threadReplies(client: Client4, rootId: string): Promise<Post[]> {
    const {order, posts} = await client.getPostThread(rootId);
    return order.map((id) => posts[id]).filter((post) => post.id !== rootId).sort((a, b) => a.create_at - b.create_at);
}
