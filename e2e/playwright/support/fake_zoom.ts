// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {IssuedToken, JournalEntry, ZoomMeeting, ZoomUser} from '../fake-zoom/state';

import {fakeZoomControlURL} from './constants';

async function control<T>(method: string, path: string, body?: unknown): Promise<T> {
    const res = await fetch(`${fakeZoomControlURL}/__control${path}`, {
        method,
        headers: {'Content-Type': 'application/json'},
        body: body === undefined ? undefined : JSON.stringify(body),
    });
    if (!res.ok) {
        throw new Error(`fake-zoom control ${method} ${path} failed: ${res.status} ${await res.text()}`);
    }
    return res.json() as Promise<T>;
}

export const fakeZoom = {
    reset: () => control('POST', '/reset'),
    requests: () => control<JournalEntry[]>('GET', '/requests'),
    calls: async () => (await fakeZoom.requests()).map((r) => `${r.method} ${r.path}`),
    contractViolations: () => control<string[]>('GET', '/contract-violations'),
    createUser: (email: string, pmi?: number) => control<ZoomUser>('POST', '/users', {email, pmi}),
    issueTokens: (userId: string, expiresInSeconds?: number) => control<IssuedToken>('POST', '/tokens', {userId, expiresInSeconds}),
    setAccountLevelApp: (accountLevel: boolean) => control('POST', '/app', {accountLevel}),
    authorizeAs: (userId: string) => control('POST', '/authorizing-user', {userId}),
    meetings: () => control<ZoomMeeting[]>('GET', '/meetings'),
    createMeeting: (hostId: string, topic: string, type = 2) => control<ZoomMeeting>('POST', '/meetings', {hostId, topic, type}),
    createDownload: (content: string) => control<{downloadURL: string; playURL: string; downloadToken: string}>('POST', '/downloads', {content}),
    fault: (method: string, path: string, status: number) => control('POST', '/fault', {method, path, status}),
};
