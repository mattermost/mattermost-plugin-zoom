import React from 'react';
import {useDispatch, useSelector} from 'react-redux';
import {Modal} from 'react-bootstrap';
import { closeScheduleMeetingModal } from '../../actions';
import { isScheduleMeetingModalVisible } from '../../selectors';

const CreateIssueModal = () => {
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
        </Modal>
    );
};

export default CreateIssueModal;
