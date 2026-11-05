// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Client4} from '@mattermost/client';
import type {Post} from '@mattermost/types/posts';

import {fakeZoomURL} from './constants';

export type MeetingPostOptions = {
    status?: 'STARTED' | 'ENDED' | 'RECENTLY_CREATED';
    topic?: string;
    meetingId?: number;
    personal?: boolean;
    creatorUsername?: string;
};

// Props must match postMeeting/postConfirm (server/http.go).
export async function createMeetingPost(client: Client4, channelId: string, {status = 'STARTED', topic = 'Zoom Meeting', meetingId = 81234567890, personal = false, creatorUsername = 'zoom-e2e'}: MeetingPostOptions = {}) {
    const props = {
        meeting_id: meetingId,
        meeting_uuid: 'e2e-meeting-uuid==',
        meeting_link: `${fakeZoomURL}/j/${meetingId}`,
        meeting_status: status === 'ENDED' ? 'STARTED' : status,
        meeting_personal: personal,
        meeting_topic: topic,
        meeting_creator_username: creatorUsername,
        meeting_provider: 'Zoom',
    };
    const message = status === 'RECENTLY_CREATED' ? `${creatorUsername} already created a Zoom call recently` : 'I have started a meeting';
    const post = await client.createPost({channel_id: channelId, message, type: 'custom_zoom', props} as unknown as Post);
    return status === 'ENDED' ? endMeetingPost(client, post) : post;
}

// Mirrors handleMeetingEnded (server/webhook.go).
export async function endMeetingPost(client: Client4, post: Post) {
    return client.patchPost({
        id: post.id,
        message: 'The meeting has ended.',
        props: {...post.props, meeting_status: 'ENDED', meeting_end_time: post.create_at + (30 * 60_000)},
    } as unknown as Post);
}

export async function createReplyPost(client: Client4, root: Post, type: 'custom_zoom_chat' | 'custom_zoom_transcript', message: string) {
    return client.createPost({channel_id: root.channel_id, root_id: root.id, message, type} as unknown as Post);
}
