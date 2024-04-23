import {Store} from 'redux';

import {openScheduleMeetingModal} from '../actions';

export function handleOpenScheduleMeetingDialog(store: Store) {
    return () => {
        openScheduleMeetingModal()(store.dispatch);
    };
}

export function handleMeetingStarted(msg: {data: {meeting_url: string}}) {
    window.open(msg.data.meeting_url);
}
