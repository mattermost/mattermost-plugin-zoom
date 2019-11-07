// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

import {Client4} from 'mattermost-redux/client';
import {ClientError} from 'mattermost-redux/client/client4';

import {id} from '../manifest';

export default class Client {
    constructor() {
        this.url = '/plugins/' + id;
    }

    startMeeting = async (channelId, personal = true, topic = '', meetingId = 0) => {
        return doPost(`${this.url}/api/v1/meetings`, {channel_id: channelId, personal, topic, meeting_id: meetingId});
    }
}

export const doPost = async (url, body, headers = {}) => {
    const options = {
        method: 'post',
        body: JSON.stringify(body),
        headers,
    }

    const response = await fetch(url, Client4.getOptions(options));

    let data = await response.json();
    if (response.ok) {
        return data;
    }

    // Error received is either a regular string, or json with the error message in data.message
    let message = data || '';
    if (data && data.message) {
        message = data.message;
    }

    throw new ClientError(Client4.url, {
        message,
        status_code: response.status,
        url,
    });
}
