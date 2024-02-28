import React, { ChangeEvent, useState } from 'react';
import {Modal} from 'react-bootstrap';
import FormButton from '../form_button';
import DatePicker from 'react-datepicker';
import 'react-datepicker/dist/react-datepicker.css';
import './schedule_meeting.css'
import {useDispatch, useSelector}  from 'react-redux';
import {scheduleMeeting} from '@/actions';
import {getCurrentChannelId} from 'mattermost-redux/selectors/entities/common';

type Props = {
    handleClose: () => void;
}

const ScheduleMeetingForm = ({handleClose}: Props) => {
    const [startDate, setStartDate] = useState(new Date());
    const [showErrors, setShowErrors] = useState(false)
    const [topic, setTopic] = useState("Zoom Meeting")
    const [durationHours, setDurationHours] = useState(0)
    const [durationMinutes, setDurationMinutes] = useState(40)
    const [meetingIdType, setMeetingIdType] = useState('personal_meeting_id')
    const [postMeetingAnnouncement, setPostMeetingAnnouncement] = useState(true)
    const [postMeetingReminder, setPostMeetingReminder] = useState(false)

    const currentChannelId = useSelector(getCurrentChannelId)

    const dispatch = useDispatch();
    const handleSchedule = (e: React.FormEvent<HTMLFormElement> | Event) => {
        e.preventDefault();

        if(!topic || !startDate || Number.isNaN(durationHours) || Number.isNaN(durationMinutes)){
            setShowErrors(true);
        }

        dispatch(scheduleMeeting({
            channelId: currentChannelId, 
            topic, 
            startTime: startDate, 
            duration: durationHours, 
            postMeetingAnnouncement, 
            postMeetingReminder, 
            meetingIdType
        }))
    }

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

    const handleTopicChange = (e: ChangeEvent<HTMLInputElement>) => setTopic(e.target.value)
    const handleDurationHourChange = (e: ChangeEvent<HTMLInputElement>) => setDurationHours(parseInt(e.target.value))
    const handleDurationMinChange = (e: ChangeEvent<HTMLInputElement>) => setDurationMinutes(parseInt(e.target.value))
    const handleMeetingIdChange = (e: ChangeEvent<HTMLInputElement>) => setMeetingIdType(e.target.value)
    const handlePostMeetingAnnouncement = (e: ChangeEvent<HTMLInputElement>) => setPostMeetingAnnouncement(e.target.checked)
    const handlePostMeetingReminder = (e: ChangeEvent<HTMLInputElement>) => setPostMeetingReminder(e.target.checked)

    return (
        <form
            role='form'
            onSubmit={handleSchedule}
        >
            <Modal.Body className='schedule-meeting_form'>
                {getRequiredLabel('Meeting Topic:')}  
                <input type='text' value={topic} onChange={handleTopicChange} className='form-control margin-bottom_15'/>
                {showErrors && (!topic && <span className='error-text'>This is required</span>)}
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
                {showErrors && (!startDate && <span className='error-text'>This is required</span>)}
                {getRequiredLabel('Meeting Duration:')}  
                <div className='display-flex margin-bottom_15'>
                    <span className='schedule-meeting_duration-input'>
                        <input type='number' min={0} max={24} value={durationHours} onChange={handleDurationHourChange} className='form-control margin-right_10'/><label className='margin-right_10'>hr</label>
                    </span>
                    <span className='schedule-meeting_duration-input'>
                        <input type='number' min={0} max={59} value={durationMinutes} onChange={handleDurationMinChange} className='form-control margin-right_10'/><label className='margin-right_10'>min</label>
                    </span>
                </div>
                {showErrors && ((Number.isNaN(durationHours) || Number.isNaN(durationMinutes))  && <p className='error-text'>This is required</p>)}
                {getRequiredLabel('Meeting ID:')}  
                <div className='display-flex margin-bottom_15'>
                    <span className='display-flex  margin-right_60'>
                        <input type='radio' className='margin-right_10' value={'personal_meeting_id'} onChange={handleMeetingIdChange} name='meeting-id-type' defaultChecked/><label className='m-0'>Personal Meeting ID</label>                              
                    </span>
                    <span className='display-flex'>
                        <input type='radio' className='margin-right_10' value={'unique_meeting_id'} onChange={handleMeetingIdChange} name='meeting-id-type'/><label className='m-0'>Unique Meeting ID</label>
                    </span>
                </div>
                <div className='display-flex'>
                    <input type='checkbox' onChange={handlePostMeetingAnnouncement}  className='margin-right_10' checked={postMeetingAnnouncement} /><label className='m-0'>Post meeting announcement to channel</label>  
                </div>
                <div className='display-flex'>
                    <input type='checkbox' onChange={handlePostMeetingReminder} className='margin-right_10' checked={postMeetingReminder} /><label className='m-0' >Post meeting reminder to channel</label>    
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
                    // saving={false}
                    defaultMessage='Schedule'
                    savingMessage='Scheduling'
                />
            </Modal.Footer>
        </form>
    );
};

export default ScheduleMeetingForm;
