import {combineReducers} from 'redux';

const isScheduleMeetingModalVisible = (state = false, action) => {
    switch (action.type) {
    case 'OPEN_SCHEDULE_MEETING_MODAL':
        return true;
    case 'CLOSE_SCHEDULE_MEETING_MODAL':
        return false;
    default:
        return state;
    }
};

export default combineReducers({
    isScheduleMeetingModalVisible,
});
