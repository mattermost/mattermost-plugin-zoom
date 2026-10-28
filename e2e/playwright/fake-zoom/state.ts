// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

export type JournalEntry = {
    method: string;
    path: string;
    query: Record<string, string>;
    headers: Record<string, string>;
    body: unknown;
};

export type ZoomUser = {
    id: string;
    email: string;
    first_name: string;
    last_name: string;
    display_name: string;
    type: number;
    pmi: number;
    timezone: string;
    verified: number;
    status: string;
    created_at: string;
    last_login_time: string;
    account_id: string;
};

export type ZoomMeeting = {
    uuid: string;
    id: number;
    host_id: string;
    host_email: string;
    topic: string;
    type: number;
    status: string;
    start_time: string;
    duration: number;
    timezone: string;
    agenda: string;
    created_at: string;
    start_url: string;
    join_url: string;
    password: string;
    h323_password: string;
    pstn_password: string;
    encrypted_password: string;
    pre_schedule: boolean;
    settings: Record<string, unknown>;
};

export type IssuedToken = {
    access_token: string;
    refresh_token: string;
    expires_in: number;
    token_type: string;
    scope: string;
    api_url: string;
};

export type TokenGrant = {
    userId: string;

    accountLevel: boolean;
    expiresAt: number;
};

export type DownloadFile = {
    token: string;
    content: string;
};

export type FakeZoomOptions = {
    zoomURL: string;
    apiURL: string;
    downloadURL: string;
    clientId: string;
    clientSecret: string;

    // Zoom refuses to authorize any other redirect URL.
    redirectURI: string;
};

export type FakeZoomState = {
    options: FakeZoomOptions;
    accountLevelApp: boolean;
    users: Map<string, ZoomUser>;
    meetings: Map<number, ZoomMeeting>;
    authorizationCodes: Map<string, {redirectURI: string; userId: string}>;
    accessTokens: Map<string, TokenGrant>;
    refreshTokens: Map<string, TokenGrant>;
    downloads: Map<string, DownloadFile>;
    authorizingUserId: string;
    journal: JournalEntry[];
    faults: Map<string, number>;
    contractViolations: string[];
};

export const accountId = 'e2e-zoom-account';

export function defaultUser(): ZoomUser {
    return newUser('e2e-zoom-user', 'e2e-zoom-user@example.com');
}

export function newUser(id: string, email: string, pmi = 0): ZoomUser {
    return {
        id,
        email,
        first_name: 'E2E',
        last_name: 'Zoom User',
        display_name: 'E2E Zoom User',
        type: 2,
        pmi,
        timezone: 'UTC',
        verified: 1,
        status: 'active',
        created_at: '2026-01-01T00:00:00Z',
        last_login_time: '2026-01-01T00:00:00Z',
        account_id: accountId,
    };
}

export function defaultState(options: FakeZoomOptions): FakeZoomState {
    const user = defaultUser();
    return {
        options,
        accountLevelApp: false,
        users: new Map([[user.id, user]]),
        meetings: new Map(),
        authorizationCodes: new Map(),
        accessTokens: new Map(),
        refreshTokens: new Map(),
        downloads: new Map(),
        authorizingUserId: user.id,
        journal: [],
        faults: new Map(),
        contractViolations: [],
    };
}
