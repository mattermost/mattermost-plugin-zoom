// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

/* eslint-disable no-console */

import {randomBytes, randomInt} from 'node:crypto';
import fs from 'node:fs';
import {createServer, IncomingMessage, Server, ServerResponse} from 'node:http';
import {createServer as createTLSServer} from 'node:https';
import path from 'node:path';

import {ContractValidator} from './contract/validator';
import {defaultState, FakeZoomOptions, FakeZoomState, IssuedToken, JournalEntry, newUser, TokenGrant, ZoomMeeting, ZoomUser} from './state';

type RequestContext = {
    req: IncomingMessage;
    res: ServerResponse;
    url: URL;
    body: string;
    params: string[];
    state: FakeZoomState;
};

type Route = [method: string, pattern: RegExp, handler: (ctx: RequestContext) => void];

// Zoom serves OAuth on zoom.us and the REST API on api.zoom.us/v2; both listeners share these routes.
const routes: Route[] = [
    ['GET', /^\/oauth\/authorize$/, ({res, url, state}) => {
        const params = url.searchParams;
        if (params.get('client_id') !== state.options.clientId || params.get('response_type') !== 'code') {
            sendJSON(res, 400, {reason: 'Invalid client_id or response_type', error: 'invalid_request'});
            return;
        }
        const redirectURI = params.get('redirect_uri') ?? '';
        if (redirectURI !== state.options.redirectURI) {
            res.writeHead(400, {'Content-Type': 'text/html'}).end('<!DOCTYPE html><title>Zoom</title><p>Invalid redirect (4,700)</p>');
            return;
        }
        const code = `code-${randomHex()}`;
        state.authorizationCodes.set(code, {redirectURI, userId: state.authorizingUserId});
        redirect(res, redirectURI, {code, state: params.get('state') ?? ''});
    }],

    ['POST', /^\/oauth\/token$/, ({req, res, url, body, state}) => {
        // Zoom accepts the grant parameters in either the query string or the form body.
        const form = new URLSearchParams(body);
        for (const [key, value] of url.searchParams) {
            if (!form.has(key)) {
                form.set(key, value);
            }
        }
        const [clientId, clientSecret] = clientCredentials(req, form);
        if (clientId !== state.options.clientId || clientSecret !== state.options.clientSecret) {
            sendJSON(res, 401, {reason: 'Invalid client_id or client_secret', error: 'invalid_client'});
            return;
        }

        if (form.get('grant_type') === 'refresh_token') {
            const grant = state.refreshTokens.get(form.get('refresh_token') ?? '');
            if (!grant) {
                sendJSON(res, 400, {reason: 'Invalid Token!', error: 'invalid_grant'});
                return;
            }
            sendJSON(res, 200, issueToken(state, grant.userId, grant.accountLevel));
            return;
        }

        const pending = state.authorizationCodes.get(form.get('code') ?? '');
        if (form.get('grant_type') !== 'authorization_code' || !pending || pending.redirectURI !== form.get('redirect_uri')) {
            sendJSON(res, 400, {reason: 'Invalid authorization code', error: 'invalid_grant'});
            return;
        }
        state.authorizationCodes.delete(form.get('code')!);
        sendJSON(res, 200, issueToken(state, pending.userId, state.accountLevelApp));
    }],

    ['GET', /^\/j\/(\d+)$/, ({res, params}) => {
        res.writeHead(200, {'Content-Type': 'text/html'}).end(`<!DOCTYPE html><title>Fake Zoom</title><h1>Fake Zoom meeting ${params[0]}</h1>`);
    }],

    ['GET', /^\/v2\/users\/([^/]+)$/, (ctx) => withGrant(ctx, (caller, grant) => {
        const target = lookupUser(ctx, caller, grant, ctx.params[0]);
        if (target) {
            sendJSON(ctx.res, 200, target);
        }
    })],

    ['POST', /^\/v2\/users\/([^/]+)\/meetings$/, (ctx) => withGrant(ctx, (caller, grant) => {
        const host = lookupUser(ctx, caller, grant, ctx.params[0]);
        if (!host) {
            return;
        }
        const request = JSON.parse(ctx.body || '{}') as {topic?: string; type?: number};
        const meeting = createMeeting(ctx.state, host, request.topic ?? 'Zoom Meeting', request.type ?? 2);
        sendJSON(ctx.res, 201, meeting);
    })],

    ['GET', /^\/v2\/meetings\/(\d+)$/, (ctx) => withGrant(ctx, () => {
        const meeting = ctx.state.meetings.get(Number(ctx.params[0]));
        if (!meeting) {
            sendJSON(ctx.res, 404, {code: 3001, message: `Meeting does not exist: ${ctx.params[0]}.`});
            return;
        }
        sendJSON(ctx.res, 200, meeting);
    })],

    ['GET', /^\/rec\/(?:download|play)\/([\w-]+)$/, ({req, res, params, state}) => {
        const file = state.downloads.get(params[0]);
        if (!file || req.headers.authorization !== `Bearer ${file.token}`) {
            sendJSON(res, 401, invalidAccessToken);
            return;
        }
        res.writeHead(200, {'Content-Type': 'text/plain'}).end(file.content);
    }],
];

const invalidAccessToken = {code: 124, message: 'Invalid access token.'};

const controlRoutes: Record<string, (ctx: RequestContext) => void> = {
    'POST /__control/reset': ({res, state}) => {
        Object.assign(state, defaultState(state.options));
        sendJSON(res, 200, {ok: true});
    },
    'GET /__control/requests': ({res, state}) => sendJSON(res, 200, state.journal),
    'GET /__control/contract-violations': ({res, state}) => sendJSON(res, 200, state.contractViolations),
    'POST /__control/users': ({res, body, state}) => {
        const {email, pmi} = JSON.parse(body) as {email: string; pmi?: number};
        const user = newUser(`zu-${randomHex()}`, email, pmi);
        state.users.set(user.id, user);
        sendJSON(res, 200, user);
    },
    'POST /__control/tokens': ({res, body, state}) => {
        const {userId, expiresInSeconds} = JSON.parse(body) as {userId: string; expiresInSeconds?: number};
        sendJSON(res, 200, issueToken(state, userId, state.accountLevelApp, expiresInSeconds));
    },
    'POST /__control/app': ({res, body, state}) => {
        state.accountLevelApp = (JSON.parse(body) as {accountLevel: boolean}).accountLevel;
        sendJSON(res, 200, {ok: true});
    },
    'POST /__control/authorizing-user': ({res, body, state}) => {
        state.authorizingUserId = (JSON.parse(body) as {userId: string}).userId;
        sendJSON(res, 200, {ok: true});
    },
    'GET /__control/meetings': ({res, state}) => sendJSON(res, 200, [...state.meetings.values()]),
    'POST /__control/meetings': ({res, body, state}) => {
        const {hostId, topic, type} = JSON.parse(body) as {hostId: string; topic: string; type: number};
        const host = state.users.get(hostId);
        if (!host) {
            sendJSON(res, 400, {error: `unknown host ${hostId}`});
            return;
        }
        sendJSON(res, 200, createMeeting(state, host, topic, type));
    },
    'POST /__control/downloads': ({res, body, state}) => {
        const {content} = JSON.parse(body) as {content: string};
        const id = randomHex();
        const token = `dl-${randomHex()}`;
        state.downloads.set(id, {token, content});
        sendJSON(res, 200, {downloadURL: `${state.options.downloadURL}/rec/download/${id}`, playURL: `${state.options.downloadURL}/rec/play/${id}`, downloadToken: token});
    },
    'POST /__control/fault': ({res, body, state}) => {
        const {method, path: faultPath, status} = JSON.parse(body) as {method: string; path: string; status: number};
        state.faults.set(`${method.toUpperCase()} ${faultPath}`, status);
        sendJSON(res, 200, {ok: true});
    },
};

export type FakeZoom = {
    close: () => Promise<void>;
};

type StartOptions = FakeZoomOptions & {port: number; tlsPort: number; certDir: string};

export async function startFakeZoom({port, tlsPort, certDir, ...options}: StartOptions): Promise<FakeZoom> {
    const state = defaultState(options);
    const contract = new ContractValidator();

    const handleRequest = async (req: IncomingMessage, res: ServerResponse) => {
        const url = new URL(req.url ?? '/', 'http://fake-zoom');
        const body = await readBody(req);
        const ctx: RequestContext = {req, res, url, body, params: [], state};

        const control = controlRoutes[`${req.method} ${url.pathname}`];
        if (control) {
            control(ctx);
            return;
        }

        const entry = toJournalEntry(req, url, body);
        state.journal.push(entry);

        const fault = state.faults.get(`${req.method} ${url.pathname}`);
        if (fault) {
            sendJSON(res, fault, {code: fault, message: `fault injected for ${req.method} ${url.pathname}`});
            return;
        }

        const apiPath = url.pathname.startsWith('/v2/') ? url.pathname.slice('/v2'.length) : undefined;
        if (apiPath) {
            state.contractViolations.push(...contract.validateRequest(req.method ?? '', apiPath, entry.body));
            checkResponseContract(res, (status, payload) => {
                state.contractViolations.push(...contract.validateResponse(req.method ?? '', apiPath, status, payload));
            });
        }

        for (const [method, pattern, handler] of routes) {
            const match = req.method === method ? pattern.exec(url.pathname) : null;
            if (match) {
                handler({...ctx, params: match.slice(1)});
                return;
            }
        }
        console.warn(`[fake-zoom] unhandled ${req.method} ${url.pathname}`);
        sendJSON(res, 404, {code: 404, message: `fake-zoom has no route for ${req.method} ${url.pathname}`});
    };

    // A throw in a handler would otherwise be an unhandled rejection, which kills the fake for the rest of the run.
    const listener = (req: IncomingMessage, res: ServerResponse) => {
        handleRequest(req, res).catch((err) => {
            console.error(`[fake-zoom] ${req.method} ${req.url} failed:`, err);
            if (res.headersSent) {
                res.end();
                return;
            }
            sendJSON(res, 500, {code: 500, message: `fake-zoom error: ${err instanceof Error ? err.message : String(err)}`});
        });
    };

    const keyPath = path.join(certDir, 'fake-zoom.key');
    const certPath = path.join(certDir, 'trust/fake-zoom.pem');
    if (!fs.existsSync(keyPath) || !fs.existsSync(certPath)) {
        throw new Error(`fake-zoom certificate missing in ${certDir}; run \`npm run env:up\` (it runs scripts/fake_zoom_cert.sh)`);
    }

    const servers: Server[] = [
        createServer(listener),
        createTLSServer({key: fs.readFileSync(keyPath), cert: fs.readFileSync(certPath)}, listener),
    ];
    await Promise.all(servers.map((server, i) => new Promise<void>((resolve) => server.listen(i === 0 ? port : tlsPort, resolve))));
    console.log(`[fake-zoom] listening on :${port} as ${options.zoomURL} and :${tlsPort} as ${options.apiURL}`);

    return {
        close: async () => {
            await Promise.all(servers.map((server) => new Promise((done) => server.close(done))));
        },
    };
}

function withGrant(ctx: RequestContext, handle: (caller: ZoomUser, grant: TokenGrant) => void) {
    const header = ctx.req.headers.authorization ?? '';
    const grant = header.toLowerCase().startsWith('bearer ') ? ctx.state.accessTokens.get(header.slice('Bearer '.length)) : undefined;
    const caller = grant ? ctx.state.users.get(grant.userId) : undefined;
    if (!grant || !caller || grant.expiresAt <= Date.now()) {
        sendJSON(ctx.res, 401, invalidAccessToken);
        return;
    }
    handle(caller, grant);
}

// Zoom only lets user-level tokens address the caller as `me`; any other user ID or email is rejected as an
// invalid token. Account-level tokens can address any user on the account by ID or email.
function lookupUser(ctx: RequestContext, caller: ZoomUser, grant: TokenGrant, rawId: string) {
    const id = decodeURIComponent(rawId);
    if (id === 'me') {
        return caller;
    }
    if (!grant.accountLevel) {
        sendJSON(ctx.res, 401, invalidAccessToken);
        return undefined;
    }
    const user = ctx.state.users.get(id) ?? [...ctx.state.users.values()].find((u) => u.email.toLowerCase() === id.toLowerCase());
    if (!user || user.account_id !== caller.account_id) {
        sendJSON(ctx.res, 404, {code: 1001, message: `User does not exist: ${id}.`});
        return undefined;
    }
    return user;
}

function createMeeting(state: FakeZoomState, host: ZoomUser, topic: string, type: number): ZoomMeeting {
    const id = randomInt(80_000_000_000, 89_999_999_999);
    const password = randomHex().slice(0, 6);
    const meeting: ZoomMeeting = {
        uuid: randomBytes(16).toString('base64'),
        id,
        host_id: host.id,
        host_email: host.email,
        topic,
        type,
        status: 'waiting',
        start_time: new Date().toISOString().replace(/\.\d+Z$/, 'Z'),
        duration: 60,
        timezone: 'UTC',
        agenda: '',
        created_at: new Date().toISOString().replace(/\.\d+Z$/, 'Z'),
        start_url: `${state.options.zoomURL}/s/${id}?zak=${randomHex()}`,
        join_url: `${state.options.zoomURL}/j/${id}?pwd=${password}`,
        password,
        h323_password: password,
        pstn_password: password,
        encrypted_password: password,
        pre_schedule: false,
        settings: {
            host_video: false,
            participant_video: false,
            join_before_host: false,
            mute_upon_entry: false,
            watermark: false,
            use_pmi: false,
            approval_type: 2,
            audio: 'voip',
            auto_recording: 'none',
            waiting_room: false,
        },
    };
    state.meetings.set(id, meeting);
    return meeting;
}

// Zoom keeps one live token pair per user and app: a new pair (from a code exchange or a refresh) revokes the old one.
function issueToken(state: FakeZoomState, userId: string, accountLevel: boolean, expiresIn = 3599): IssuedToken {
    for (const tokens of [state.accessTokens, state.refreshTokens]) {
        for (const [token, grant] of tokens) {
            if (grant.userId === userId) {
                tokens.delete(token);
            }
        }
    }
    const accessToken = `acc-${randomHex()}`;
    const refreshToken = `ref-${randomHex()}`;
    state.accessTokens.set(accessToken, {userId, accountLevel, expiresAt: Date.now() + (expiresIn * 1000)});
    state.refreshTokens.set(refreshToken, {userId, accountLevel, expiresAt: Date.now() + (90 * 24 * 3600 * 1000)});
    return {
        access_token: accessToken,
        refresh_token: refreshToken,
        expires_in: expiresIn,
        token_type: 'bearer',
        scope: accountLevel ? 'meeting:write:admin user:read:admin' : 'meeting:write user:read',
        api_url: new URL(state.options.apiURL).origin,
    };
}

// golang.org/x/oauth2 auto-detects the auth style: Basic auth header first, then form fields.
function clientCredentials(req: IncomingMessage, form: URLSearchParams): [string, string] {
    const header = req.headers.authorization ?? '';
    if (header.startsWith('Basic ')) {
        const [id, secret] = Buffer.from(header.slice('Basic '.length), 'base64').toString().split(':');
        return [decodeURIComponent(id), decodeURIComponent(secret ?? '')];
    }
    return [form.get('client_id') ?? '', form.get('client_secret') ?? ''];
}

function checkResponseContract(res: ServerResponse, check: (status: number, payload: unknown) => void) {
    const end = res.end.bind(res);
    res.end = ((chunk?: unknown, ...rest: unknown[]) => {
        if (typeof chunk === 'string') {
            try {
                check(res.statusCode, JSON.parse(chunk));
            } catch {
                // non-JSON responses have no schema to check
            }
        }
        return (end as (...args: unknown[]) => ServerResponse)(chunk, ...rest);
    }) as typeof res.end;
}

function toJournalEntry(req: IncomingMessage, url: URL, body: string): JournalEntry {
    let parsed: unknown = body;
    try {
        parsed = body ? JSON.parse(body) : undefined;
    } catch {
        // non-JSON bodies are journaled as raw text
    }
    const headers: Record<string, string> = {};
    if (req.headers.authorization) {
        headers.authorization = req.headers.authorization;
    }
    return {method: req.method ?? '', path: url.pathname, query: Object.fromEntries(url.searchParams), headers, body: parsed};
}

function readBody(req: IncomingMessage): Promise<string> {
    return new Promise((resolve, reject) => {
        let data = '';
        req.on('data', (chunk) => {
            data += chunk;
        });
        req.on('end', () => resolve(data));
        req.on('error', reject);
    });
}

function redirect(res: ServerResponse, target: string, params: Record<string, string>) {
    const location = new URL(target);
    for (const [key, value] of Object.entries(params)) {
        location.searchParams.set(key, value);
    }
    res.writeHead(302, {Location: location.toString()}).end();
}

function sendJSON(res: ServerResponse, status: number, payload: unknown) {
    res.writeHead(status, {'Content-Type': 'application/json'}).end(JSON.stringify(payload));
}

function randomHex() {
    return randomBytes(8).toString('hex');
}
