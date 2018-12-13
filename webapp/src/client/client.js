// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

import request from 'superagent';

import {id} from '../manifest';

export default class Client {
    constructor() {
        this.url = '/plugins/' + id;
    }

    startMeeting = async (channelId, personal = true, topic = '', meetingId = 0) => {
        return this.doPost(`${this.url}/api/v1/meetings`, {channel_id: channelId, personal, topic, meeting_id: meetingId});
    }

    doPost = async (url, body, headers = {}) => {
        headers['X-Requested-With'] = 'XMLHttpRequest';

        try {
            const response = await request.
                post(url).
                send(body).
                set(headers).
                type('application/json').
                accept('application/json');

            return response.body;
        } catch (err) {
            throw err;
        }
    }
}
