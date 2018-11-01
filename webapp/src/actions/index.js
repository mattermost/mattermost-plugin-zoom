// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

import {PostTypes} from 'mattermost-redux/action_types';

import Client from '../client';

export function startMeeting(channelId) {
    return async (dispatch, getState) => {
        try {
            await Client.startMeeting(channelId, true);
        } catch (error) {
            const post = {
                id: 'zoomPlugin' + Date.now(),
                create_at: Date.now(),
                update_at: 0,
                edit_at: 0,
                delete_at: 0,
                is_pinned: false,
                user_id: getState().entities.users.currentUserId,
                channel_id: channelId,
                root_id: '',
                parent_id: '',
                original_id: '',
                message: 'We could not verify your Mattermost account in Zoom. Please ensure that your Mattermost email address matches your Zoom email address.',
                type: 'system_ephemeral',
                props: {},
                hashtags: '',
                pending_post_id: '',
            };

            dispatch({
                type: PostTypes.RECEIVED_POSTS,
                data: {
                    order: [],
                    posts: {
                        [post.id]: post,
                    },
                },
                channelId,
            });

            return {error};
        }

        return {data: true};
    };
}
