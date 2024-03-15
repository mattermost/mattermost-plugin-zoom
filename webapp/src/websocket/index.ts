import {Store} from 'redux';

import {openScheduleMeetingModal} from '../actions';

export function handleOpenScheduleMeetingDialog(store: Store) {
    return () => {
        openScheduleMeetingModal()(store.dispatch);
    };
}
