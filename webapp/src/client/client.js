// Copyright (c) 2019-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {Client4} from 'mattermost-redux/client';
import {ClientError} from 'mattermost-redux/client/client4';

import manifest from '../manifest';

export default class Client {
    setServerRoute(url) {
        this.url = url + '/plugins/' + manifest.id;
    }

    startMeeting = async (channelId, rootId, topic = '', force = false) => {
        const res = await doPost(`${this.url}/api/v1/meetings${force ? '?force=true' : ''}`, {
            channel_id: channelId,
            topic,
            root_id: rootId,
        });

        return {meetingUrl: res.meeting_url, error: res.error};
    }

    getChannelIdForThread = async (baseURL, threadId) => {
        const threadDetails = await doGet(`${baseURL}/api/v4/posts/${threadId}/thread`);
        return threadDetails.posts[threadId].channel_id;
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
        return response.json();
    }

    const text = await response.text();

    throw new ClientError(Client4.url, {
        message: text || '',
        status_code: response.status,
        url,
    });
};

export const doGet = async (url) => {
    const options = {
        method: 'get',
    };

    const response = await fetch(url, Client4.getOptions(options));

    if (response.ok) {
        return response.json();
    }

    const text = await response.text();

    throw new ClientError(Client4.url, {
        message: text || '',
        status_code: response.status,
        url,
    });
};
