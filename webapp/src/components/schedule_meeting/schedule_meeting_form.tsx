import React, { useState } from 'react';
import {Modal} from 'react-bootstrap';
import FormButton from '../form_button';
import DatePicker from 'react-datepicker';
import 'react-datepicker/dist/react-datepicker.css';
import './schedule_meeting.css'

type Props = {
    handleClose: () => void;
}

const ScheduleMeetingForm = ({handleClose}: Props) => {
    const [startDate, setStartDate] = useState(new Date());

    const getRequiredLabel = (label: string) => (
        <span>
            <label>{label}</label>   
            <span
                className='error-text'
                style={{marginLeft: '3px'}}
            >
                {'*'}
            </span>
        </span>
    )

    return (
        <form>
            <Modal.Body className='schedule-meeting_form'>
                {getRequiredLabel('Meeting Topic:')}  
                <input type='text' defaultValue={'Zoom Meeting'} className='form-control margin-bottom_15'/>
                {getRequiredLabel('Meeting Date & Time:')}  
                <DatePicker
                    className='form-control margin-bottom_15'
                    showIcon
                    selected={startDate}
                    onChange={(date: any) => setStartDate(date)}
                    timeInputLabel='Start Time:'
                    dateFormat='MM/dd/yyyy h:mm aa'
                    showTimeInput
                    calendarIconClassname='meeting-calendar_icon'
                    calendarClassName='meeting-calendar'
                    weekDayClassName={(_) => 'margin_5'}
                    dayClassName={(_) => 'margin_5'}                    
                />
                {getRequiredLabel('Meeting Duration:')}  
                <div className='display-flex margin-bottom_15'>
                    <span className='schedule-meeting_duration-input'>
                        <input type='number' min={0} max={24} defaultValue={0} className='form-control margin-right_10'/><label className='margin-right_10'>hr</label>
                    </span>
                    <span className='schedule-meeting_duration-input'>
                        <input type='number' min={0} max={59} defaultValue={40} className='form-control margin-right_10'/><label className='margin-right_10'>min</label>
                    </span>
                </div>
                {getRequiredLabel('Meeting ID:')}  
                <div className='display-flex margin-bottom_15'>
                    <span className='display-flex  margin-right_60'>
                        <input type='radio' className='margin-right_10' name='meeting-id' defaultChecked/><label className='m-0'>Personal Meeting ID</label>                              
                    </span>
                    <span className='display-flex'>
                        <input type='radio' className='margin-right_10' name='meeting-id'/><label className='m-0'>Unique Meeting ID</label>                        
                    </span>
                </div>
                <div className='display-flex'>
                    <input type='checkbox' className='margin-right_10'/><label className='m-0'>Post meeting announcement to channel</label>  
                </div>
                <div className='display-flex'>
                    <input type='checkbox' className='margin-right_10' defaultChecked/><label className='m-0'>Post meeting reminder to channel</label>    
                </div>
            </Modal.Body>
            <Modal.Footer>
                <FormButton
                    btnClass='btn-link'
                    defaultMessage='Cancel'
                    onClick={handleClose}
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
