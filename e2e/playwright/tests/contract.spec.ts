// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {expect, test} from '@playwright/test';

import {ContractValidator} from '../fake-zoom/contract/validator';
import {defaultUser} from '../fake-zoom/state';

const validator = new ContractValidator();

test('fake Zoom user matches the published user schema', () => {
    expect(validator.validateResponse('GET', '/users/me', 200, defaultUser())).toEqual([]);
});

test('validator rejects fields with the wrong type', () => {
    const errors = validator.validateResponse('GET', '/users/me', 200, {...defaultUser(), pmi: 'not-a-number'});

    expect(errors.join('\n')).toContain('/pmi must be integer');
});

test('synthetic webhook events must match Zoom\'s published event schema', () => {
    const event = {event: 'meeting.started', event_ts: Date.now(), payload: {account_id: 'a', object: {id: '1', uuid: 'u', host_id: 'h', topic: 't', type: 1, start_time: '2026-01-01T00:00:00Z', duration: 0}}};

    expect(validator.validateEvent(event).join('\n')).toContain('must have required property \'host_email\'');
    expect(validator.validateEvent({...event, event: 'meeting.not_a_real_event'})).toEqual([
        'webhook event meeting.not_a_real_event is not covered by fake-zoom/contract/operations.json',
    ]);
});

test('validator flags routes missing from the contract', () => {
    expect(validator.validateRequest('GET', '/not-a-real-endpoint', undefined)).toEqual([
        'GET /not-a-real-endpoint is not covered by fake-zoom/contract/operations.json',
    ]);
});
