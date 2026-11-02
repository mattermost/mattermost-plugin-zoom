// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {expect, test} from '@playwright/test';

import {ContractValidator} from '../fake-zoom/contract/validator';
import {defaultUser} from '../fake-zoom/state';

const validator = new ContractValidator();

/**
 * @objective Verify fake-zoom's default user matches Zoom's published user schema.
 */
test('validates fake Zoom user against the published user schema', {tag: '@zoom_plugin'}, () => {
    // * Verify the default user has no contract violations
    expect(validator.validateResponse('GET', '/users/me', 200, defaultUser())).toEqual([]);
});

/**
 * @objective Verify the contract validator catches fields with the wrong type.
 */
test('rejects fields with the wrong type', {tag: '@zoom_plugin'}, () => {
    // # Validate a user whose PMI is a string
    const errors = validator.validateResponse('GET', '/users/me', 200, {...defaultUser(), pmi: 'not-a-number'});

    // * Verify the type error is reported
    expect(errors.join('\n')).toContain('/pmi must be integer');
});

/**
 * @objective Verify auth failures are checked against Zoom's API-wide error shape.
 */
test('validates auth failures against Zoom API-wide error shape', {tag: '@zoom_plugin'}, () => {
    // * Verify a well-formed 401 passes and malformed or undocumented errors fail
    expect(validator.validateResponse('GET', '/users/me', 401, {code: 124, message: 'Invalid access token.'})).toEqual([]);
    expect(validator.validateResponse('GET', '/users/me', 401, {error: 'unauthorized'})).not.toEqual([]);
    expect(validator.validateResponse('GET', '/users/me', 403, {code: 124, message: 'Forbidden'})).not.toEqual([]);
});

/**
 * @objective Verify synthetic webhook events are checked against Zoom's published event schemas.
 */
test('rejects synthetic webhook events that do not match Zoom event schema', {tag: '@zoom_plugin'}, () => {
    // # Build a meeting.started event without host_email
    const event = {event: 'meeting.started', event_ts: Date.now(), payload: {account_id: 'a', object: {id: '1', uuid: 'u', host_id: 'h', topic: 't', type: 1, start_time: '2026-01-01T00:00:00Z', duration: 0}}};

    // * Verify the missing field and an unknown event name are both reported
    expect(validator.validateEvent(event).join('\n')).toContain('must have required property \'host_email\'');
    expect(validator.validateEvent({...event, event: 'meeting.not_a_real_event'})).toEqual([
        'webhook event meeting.not_a_real_event is not covered by fake-zoom/contract/operations.json',
    ]);
});

/**
 * @objective Verify the contract validator flags routes missing from operations.json.
 */
test('flags routes missing from the contract', {tag: '@zoom_plugin'}, () => {
    // * Verify an unlisted route is reported
    expect(validator.validateRequest('GET', '/not-a-real-endpoint', undefined)).toEqual([
        'GET /not-a-real-endpoint is not covered by fake-zoom/contract/operations.json',
    ]);
});
