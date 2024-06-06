// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

import {PostTypes} from 'mattermost-redux/action_types';

import Client from '../client';

export function startMeeting(channelId, rootId = '', force = false, topic = '') {
    return async (dispatch, getState) => {
        const userId = getState().entities.bots.accounts.user_id;
        try {
            const {meetingUrl, error} = await Client.startMeeting(channelId, rootId, topic, force);
            if (meetingUrl) {
                window.open(meetingUrl);
            } else if (error) {
                dispatchError(dispatch, channelId, rootId, userId, error);
                return error;
            }
        } catch (error) {
            let m = 'Error occurred while starting the Zoom meeting.';
            if (error.message && error.message[0] === '{') {
                const e = JSON.parse(error.message);

                // Error is from Zoom API
                if (e && e.message) {
                    m = '\nZoom error: ' + e.message;
                }
            }

            dispatchError(dispatch, channelId, rootId, userId, m);
            return {error};
        }

        return {data: true};
    };
}

function dispatchError(dispatch, channelId, rootId, userId, message) {
    const post = {
        id: 'zoomPlugin' + Date.now(),
        create_at: Date.now(),
        update_at: 0,
        edit_at: 0,
        delete_at: 0,
        is_pinned: false,
        user_id: userId,
        channel_id: channelId,
        root_id: rootId,
        parent_id: '',
        original_id: '',
        message,
        type: 'system_ephemeral',
        props: {},
        hashtags: '',
        pending_post_id: '',
    };

    dispatch({
        type: PostTypes.RECEIVED_NEW_POST,
        data: post,
        channelId,
        rootId,
    });
}
