import React from 'react';
import {Modal} from 'react-bootstrap';
import FormButton from '../form_button';

const ScheduleMeetingForm = () => {
    return (
        <form>
            <Modal.Body>      
            </Modal.Body>
            <Modal.Footer>
                <FormButton
                    btnClass='btn-link'
                    defaultMessage='Cancel'
                    onClick={() => {}}
                />
                <FormButton
                    btnClass='btn btn-primary'
                    saving={false}
                    defaultMessage='Schedule'
                    savingMessage='Scheduling'
                />
            </Modal.Footer>
        </form>
    );
};

export default ScheduleMeetingForm;
