import React from 'react';
import PropTypes from 'prop-types';

const AskPMISetting = (props) => {
    return (
        <div>
            <div>
                <button
                    className='btn btn-lg btn-primary'
                    style={props.styles.button}
                    onClick={() => props.
                        actions.
                        startMeetingWithPMI()
                    }
                >
                    {'With PMI?'}
                </button>
            </div>
            <div>
                <button
                    className='btn btn-lg btn-primary'
                    style={props.styles.button}
                    rel='noopener noreferrer'
                    target='_blank'
                    onClick={() => props.
                        actions.
                        startMeetingWithoutPMI()
                    }
                >
                    {'Without PMI?'}
                </button>
            </div>
        </div>
    );
};

export default AskPMISetting;

AskPMISetting.propTypes = {
    styles: PropTypes.object.isRequired,
    actions: PropTypes.shape({
        startMeetingWithoutPMI: PropTypes.func.isRequired,
        startMeetingWithPMI: PropTypes.func.isRequired,
    }).isRequired,
};