// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

import {Client4} from 'mattermost-redux/client';
import {ClientError} from 'mattermost-redux/client/client4';

import {id} from '../manifest';

export default class Client {
    setServerRoute(url) {
        this.url = url + '/plugins/' + id;
    }

    startMeeting = async (channelId, rootId, personal = true, topic = '', meetingId = 0, force = false) => {
        const res = await doPost(`${this.url}/api/v1/meetings${force ? '?force=true' : ''}`, {channel_id: channelId, root_id: rootId, personal, topic, meeting_id: meetingId});
        return res.meeting_url;
    }

    forceStartMeeting = async (channelId, rootId, personal = true, topic = '', meetingId = 0) => {
        const meetingUrl = await this.startMeeting(channelId, rootId, personal, topic, meetingId, true);
        return meetingUrl;
    }

    getChannelId = async (baseURL, threadId) => {
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
