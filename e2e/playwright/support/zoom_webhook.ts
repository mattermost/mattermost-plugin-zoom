// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {createHmac} from 'node:crypto';

import {ContractValidator} from '../fake-zoom/contract/validator';
import {accountId, type ZoomMeeting} from '../fake-zoom/state';

import {fakeZoomURL, mattermostURL, oauthClientId, pluginId, webhookSecret, zoomWebhookSecret} from './constants';

type DeliverOptions = {
    signingSecret?: string;
    mattermostSecret?: string;
    timestamp?: number;
};

type ZoomEvent = {event: string; [key: string]: unknown};

const contract = new ContractValidator();

// Payloads are checked against Zoom's event schemas so synthetic events can't drift from what Zoom sends.
export async function deliverWebhook(payload: ZoomEvent, options: DeliverOptions = {}) {
    return post(`/webhook?secret=${encodeURIComponent(options.mattermostSecret ?? webhookSecret)}`, payload, options);
}

export async function deliverDeauthorization(zoomUserId: string) {
    const now = Date.now();
    return post(`/deauthorization?secret=${encodeURIComponent(webhookSecret)}`, {
        event: 'app_deauthorized',
        payload: {
            account_id: accountId,
            user_id: zoomUserId,
            signature: `deauth-${now}`,
            deauthorization_time: new Date(now).toISOString(),
            client_id: oauthClientId,
            event_ts: now,
        },
    }, {});
}

async function post(pathAndQuery: string, payload: ZoomEvent, {signingSecret = zoomWebhookSecret, timestamp = Math.floor(Date.now() / 1000)}: DeliverOptions) {
    if (payload.event !== 'endpoint.url_validation') {
        const violations = contract.validateEvent(payload);
        if (violations.length) {
            throw new Error(`synthetic ${payload.event} does not match Zoom's event schema:\n${violations.join('\n')}`);
        }
    }

    const body = JSON.stringify(payload);
    return fetch(`${mattermostURL}/plugins/${pluginId}${pathAndQuery}`, {
        method: 'POST',
        headers: {
            'content-type': 'application/json; charset=utf-8',
            'user-agent': 'Zoom Marketplace/1.0a',
            clientid: oauthClientId,
            'x-zm-request-timestamp': String(timestamp),
            'x-zm-signature': `v0=${sign(signingSecret, `v0:${timestamp}:${body}`)}`,
        },
        body,
    });
}

export function sign(secret: string, data: string) {
    return createHmac('sha256', secret).update(data).digest('hex');
}

function meetingObject(meeting: ZoomMeeting, uuid = meeting.uuid) {
    return {
        id: String(meeting.id),
        uuid,
        host_id: meeting.host_id,
        host_email: meeting.host_email,
        topic: meeting.topic,
        type: meeting.type,
        start_time: meeting.start_time,
        timezone: meeting.timezone,
        duration: meeting.duration,
    };
}

export const zoomEvents = {
    urlValidation: (plainToken: string) => ({event: 'endpoint.url_validation', event_ts: Date.now(), payload: {plainToken}}),

    meetingStarted: (meeting: ZoomMeeting, uuid?: string) => ({
        event: 'meeting.started',
        event_ts: Date.now(),
        payload: {account_id: accountId, object: meetingObject(meeting, uuid)},
    }),

    meetingEnded: (meeting: ZoomMeeting, uuid?: string) => ({
        event: 'meeting.ended',
        event_ts: Date.now(),
        payload: {account_id: accountId, object: {...meetingObject(meeting, uuid), end_time: new Date().toISOString().replace(/\.\d+Z$/, 'Z')}},
    }),

    recordingCompleted: (meeting: ZoomMeeting, downloadToken: string, files: RecordingFile[], password = '') => recording('recording.completed', meeting, downloadToken, files, password),
    transcriptCompleted: (meeting: ZoomMeeting, downloadToken: string, files: RecordingFile[]) => recording('recording.transcript_completed', meeting, downloadToken, files, ''),
};

export type RecordingFile = {
    file_type: 'MP4' | 'M4A' | 'CHAT' | 'TRANSCRIPT';
    recording_type: string;
    download_url: string;
    play_url?: string;
};

const fileExtensions: Record<RecordingFile['file_type'], string> = {MP4: 'MP4', M4A: 'M4A', CHAT: 'TXT', TRANSCRIPT: 'VTT'};

function recording(event: string, meeting: ZoomMeeting, downloadToken: string, files: RecordingFile[], password: string) {
    const start = new Date(Date.now() - 60_000).toISOString().replace(/\.\d+Z$/, 'Z');
    const end = new Date().toISOString().replace(/\.\d+Z$/, 'Z');
    return {
        event,
        event_ts: Date.now(),
        download_token: downloadToken,
        payload: {
            account_id: accountId,
            object: {
                uuid: meeting.uuid,
                id: meeting.id,
                account_id: accountId,
                host_id: meeting.host_id,
                host_email: meeting.host_email,
                topic: meeting.topic,
                type: meeting.type,
                start_time: start,
                timezone: meeting.timezone,
                duration: 1,
                password,
                share_url: `${fakeZoomURL}/rec/share/${encodeURIComponent(meeting.uuid)}`,
                total_size: files.length * 1024,
                recording_count: files.length,
                recording_files: files.map((file, i) => ({
                    id: `file-${i}`,
                    meeting_id: meeting.uuid,
                    recording_start: start,
                    recording_end: end,
                    file_extension: fileExtensions[file.file_type],
                    file_size: 1024,
                    status: 'completed',
                    ...file,
                })),
            },
        },
    };
}
