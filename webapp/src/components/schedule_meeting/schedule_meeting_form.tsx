import React, { useState } from 'react';
import {Modal} from 'react-bootstrap';
import FormButton from '../form_button';
import DatePicker from "react-datepicker";
import "react-datepicker/dist/react-datepicker.css";

const ScheduleMeetingForm = () => {
    const [startDate, setStartDate] = useState(new Date());

    const requiredComponent = (
        <span
            className='error-text'
            style={{marginLeft: '3px'}}
        >
            {'*'}
        </span>
    )

    return (
        <form>
            <Modal.Body>
                <label>Meeting topic:</label>
                <input type="text" className='form-control'/>
                <DatePicker
                    className='form-control'
                    showIcon
                    selected={startDate}
                    onChange={(date: any) => setStartDate(date)}
                    timeInputLabel="Start Time:"
                    dateFormat="MM/dd/yyyy h:mm aa"
                    showTimeInput
                />
                <label>Meeting Duration:</label>   
                <input type="number" min={0} max={24} className='form-control'/>
                <input type="number" min={0} max={59} className='form-control'/>  
                <label>Meeting ID:</label>       
                <input type="radio" className=''/><label>Personal Meeting ID</label>      
                <input type="radio" className=''/><label>Unique Meeting ID</label>      
                <input type="checkbox" className=''/><label>Post meeting announcement to channel</label>  
                <input type="checkbox" className=''/><label>Post meeting reminder to channel</label>    
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
