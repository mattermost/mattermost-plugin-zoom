// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

import {PostTypes} from 'mattermost-redux/action_types';

import Client from '../client';

export function startMeeting(channelId, force = false, topic = '') {
    return async (dispatch, getState) => {
        try {
            let meetingURL = '';
            if (force) {
                meetingURL = await Client.forceStartMeeting(
                    channelId, true, '', 0, topic,
                );
            } else {
                meetingURL = await Client.startMeeting(
                    channelId, true, '', 0, false, topic,
                );
            }
            if (meetingURL) {
                window.open(meetingURL);
            }
        } catch (error) {
            let m = error.message;
            if (error.message && error.message[0] === '{') {
                const e = JSON.parse(error.message);

                // Error is from Zoom API
                if (e && e.message) {
                    m = '\nZoom error: ' + e.message;
                }
            }

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
                message: m,
                type: 'system_ephemeral',
                props: {},
                hashtags: '',
                pending_post_id: '',
            };

            dispatch({
                type: PostTypes.RECEIVED_NEW_POST,
                data: post,
                channelId,
            });

            return {error};
        }

        return {data: true};
    };
}
