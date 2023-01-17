// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

import {Client4} from 'mattermost-redux/client';
import {ClientError} from 'mattermost-redux/client/client4';

import {id} from '../manifest';

export default class Client {
    setServerRoute(url) {
        this.url = url + '/plugins/' + id;
    }

    startMeeting = async (
        channelId, personal = true, topic = '', meetingId = 0, force = false,
    ) => {
        const res = await doPost(`${this.url}/api/v1/meetings${force ? '?force=true' : ''}`, {
            channel_id: channelId,
            personal,
            topic,
            meeting_id: meetingId,
        });
        return res.meeting_url;
    }

    forceStartMeeting = async (channelId, personal = true, topic = '', meetingId = 0) => {
        const meetingUrl = await this.startMeeting(channelId, personal, topic, meetingId, true);
        return meetingUrl;
    }
}

export const doPost = async (url, body, headers = {}) => {
    const options = {
        method: 'post',
        body: JSON.stringify(body),
        headers,
    };

    const response = await fetch(url, Client4.getOptions(options));
    if (response.ok) {
        return response;
    }

    const text = await response.text();

    throw new ClientError(Client4.url, {
        message: text || '',
        status_code: response.status,
        url,
    });
};
