// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Client4} from '@mattermost/client';

import type {ZoomMeeting} from '../fake-zoom/state';

import {mattermostURL, pluginId} from './constants';
import {fakeZoom} from './fake_zoom';
import {setPMIPreference} from './kv';
import {zoomPosts} from './ui';

// Same request the channel header button makes.
export async function startMeeting(userClient: Client4, userId: string, channelId: string, topic = '') {
    setPMIPreference(userId, 'false');
    const res = await fetch(`${mattermostURL}/plugins/${pluginId}/api/v1/meetings`, {
        method: 'POST',
        headers: {Authorization: `Bearer ${userClient.getToken()}`, 'Content-Type': 'application/json'},
        body: JSON.stringify({channel_id: channelId, topic}),
    });
    if (!res.ok) {
        throw new Error(`start meeting failed: ${res.status} ${await res.text()}`);
    }

    const meetings = await fakeZoom.meetings();
    const meeting: ZoomMeeting = meetings[meetings.length - 1];
    const post = (await zoomPosts(userClient, channelId)).find((p) => p.props.meeting_id === meeting.id);
    if (!post) {
        throw new Error(`No channel post for meeting ${meeting.id}`);
    }
    return {meeting, post};
}
