import React from 'react';
import {useDispatch, useSelector} from 'react-redux';
import {Modal} from 'react-bootstrap';
import { closeScheduleMeetingModal } from '../../actions';
import { isScheduleMeetingModalVisible } from '../../selectors';
import ScheduleMeetingForm from './schedule_meeting_form';

const ScheduleMeetingModal = () => {
    const dispatch = useDispatch();
    const handleClose = () => {
        dispatch(closeScheduleMeetingModal());
    };

    const visible = useSelector(isScheduleMeetingModalVisible);    
    if (!visible) {
        return null;
    }

    return (
        <Modal
            dialogClassName='modal--scroll'
            show={true}
            onHide={handleClose}
            onExited={handleClose}
            bsSize='large'
            backdrop='static'
        >
            <Modal.Header closeButton={true}>
                <Modal.Title>
                    {'Schedule Zoom Meeting'}
                </Modal.Title>
            </Modal.Header>
            <ScheduleMeetingForm
                handleClose={handleClose}
            />
        </Modal>
    );
};

export default ScheduleMeetingModal;
