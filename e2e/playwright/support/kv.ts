// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {execFileSync} from 'node:child_process';
import {createCipheriv, createDecipheriv, randomBytes} from 'node:crypto';
import path from 'node:path';

import type {IssuedToken, ZoomUser} from '../fake-zoom/state';

import {encryptionKey, pluginId} from './constants';

export type PMIPreference = 'true' | 'false' | 'ask';

type StoredToken = {access_token: string; token_type: string; refresh_token: string; expiry: string};

export type StoredConnection = {
    ZoomEmail: string;
    ZoomID: string;
    UserID: string;
    OAuthToken: StoredToken;
};

const composeDir = path.join(__dirname, '..');

// Keys and format must match storeOAuthUserInfo (server/store.go).
export function seedConnection(mattermostUserId: string, zoomUser: ZoomUser, token: IssuedToken, expiresInSeconds = token.expires_in) {
    const connection: StoredConnection = {
        ZoomEmail: zoomUser.email,
        ZoomID: zoomUser.id,
        UserID: mattermostUserId,
        OAuthToken: {
            access_token: encrypt(token.access_token),
            token_type: token.token_type,
            refresh_token: token.refresh_token,
            expiry: new Date(Date.now() + (expiresInSeconds * 1000)).toISOString(),
        },
    };
    upsert({
        [`zoomtoken_${mattermostUserId}`]: connection,
        [`zoomtokenbyzoomid_${zoomUser.id}`]: connection,
    });
}

export function readConnection(mattermostUserId: string): StoredConnection | undefined {
    const raw = get(`zoomtoken_${mattermostUserId}`);
    if (!raw) {
        return undefined;
    }
    const connection = JSON.parse(raw) as StoredConnection;
    return {...connection, OAuthToken: {...connection.OAuthToken, access_token: decrypt(connection.OAuthToken.access_token)}};
}

export function hasKey(key: string) {
    return Boolean(get(key));
}

export const superUserTokenKey = 'zoomSuperUserToken_';

// Same format as setSuperUserToken (server/store.go).
export function seedSuperUserToken(token: IssuedToken) {
    upsert({
        [superUserTokenKey]: {
            access_token: token.access_token,
            token_type: token.token_type,
            refresh_token: token.refresh_token,
            expiry: new Date(Date.now() + (token.expires_in * 1000)).toISOString(),
        },
    });
}

export function deleteKey(key: string) {
    psql(`DELETE FROM pluginkeyvaluestore WHERE pluginid = '${pluginId}' AND pkey = ${quote(key)};`);
}

export function setPMIPreference(mattermostUserId: string, value: PMIPreference) {
    upsert({[`zoomUserPreference_${mattermostUserId}`]: value});
}

function get(key: string) {
    return psql(`SELECT convert_from(pvalue, 'UTF8') FROM pluginkeyvaluestore WHERE pluginid = '${pluginId}' AND pkey = ${quote(key)};`, ['-A', '-t']).trim();
}

function upsert(entries: Record<string, unknown>) {
    const values = Object.entries(entries).map(([key, value]) => `('${pluginId}', ${quote(key)}, convert_to(${quote(JSON.stringify(value))}, 'UTF8'), 0)`);
    psql(`INSERT INTO pluginkeyvaluestore (pluginid, pkey, pvalue, expireat) VALUES ${values.join(', ')}
        ON CONFLICT (pluginid, pkey) DO UPDATE SET pvalue = EXCLUDED.pvalue, expireat = 0;`);
}

function psql(sql: string, flags: string[] = []) {
    return execFileSync('docker', ['compose', 'exec', '-T', 'postgres', 'psql', '-U', 'mmuser', '-d', 'mattermost_test', '-v', 'ON_ERROR_STOP=1', '-q', ...flags], {
        cwd: composeDir,
        input: sql,
        encoding: 'utf8',
        stdio: ['pipe', 'pipe', 'inherit'],
    });
}

function quote(value: string) {
    return `'${value.replaceAll("'", "''")}'`;
}

// Mirrors server/cipher.go: PKCS#7 padding, AES-CFB with a random IV prefix, padded URL-safe base64.
function encrypt(text: string) {
    const iv = randomBytes(16);
    const plain = Buffer.from(text);
    const padding = 16 - (plain.length % 16);
    const cipher = createCipheriv('aes-256-cfb', Buffer.from(encryptionKey), iv);
    const body = Buffer.concat([cipher.update(Buffer.concat([plain, Buffer.alloc(padding, padding)])), cipher.final()]);
    return Buffer.concat([iv, body]).toString('base64').replaceAll('+', '-').replaceAll('/', '_');
}

function decrypt(text: string) {
    const data = Buffer.from(text.replaceAll('-', '+').replaceAll('_', '/'), 'base64');
    const decipher = createDecipheriv('aes-256-cfb', Buffer.from(encryptionKey), data.subarray(0, 16));
    const plain = Buffer.concat([decipher.update(data.subarray(16)), decipher.final()]);
    return plain.subarray(0, plain.length - plain[plain.length - 1]).toString();
}
