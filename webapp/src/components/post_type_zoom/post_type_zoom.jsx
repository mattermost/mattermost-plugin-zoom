// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

import React from 'react';
import PropTypes from 'prop-types';

import {makeStyleFromTheme} from 'mattermost-redux/utils/theme_utils';

import {Svgs} from '../../constants';
import {formatDate} from '../../utils/date_utils';

export default class PostTypeZoom extends React.PureComponent {
    static propTypes = {

        /*
         * The post to render the message for.
         */
        post: PropTypes.object.isRequired,

        /**
         * Set to render post body compactly.
         */
        compactDisplay: PropTypes.bool,

        /**
         * Flags if the post_message_view is for the RHS (Reply).
         */
        isRHS: PropTypes.bool,

        /**
         * Set to display times using 24 hours.
         */
        useMilitaryTime: PropTypes.bool,

        /*
         * Logged in user's theme.
         */
        theme: PropTypes.object.isRequired,

        /*
         * Creator's name.
         */
        creatorName: PropTypes.string.isRequired,

        /*
         * Current Channel Id.
         */
        currentChannelId: PropTypes.string.isRequired,

        /*
         * Whether the post was sent from a bot. Used for backwards compatibility.
         */
        fromBot: PropTypes.bool,

        actions: PropTypes.shape({
            startMeeting: PropTypes.func.isRequired,
        }).isRequired,
    };

    static defaultProps = {
        mentionKeys: [],
        compactDisplay: false,
        isRHS: false,
    };

    constructor(props) {
        super(props);

        this.state = {
        };
    }

    render() {
        const style = getStyle(this.props.theme);
        const post = this.props.post;
        const props = post.props || {};

        let preText;
        let content;
        let subtitle;
        if (props.meeting_status === 'STARTED') {
            preText = 'I have started a meeting';
            if (this.props.fromBot) {
                preText = `${this.props.creatorName} has started a meeting`;
            }
            content = (
                <a
                    className='btn btn-lg btn-primary'
                    style={style.button}
                    rel='noopener noreferrer'
                    target='_blank'
                    href={props.meeting_link}
                >
                    <i
                        style={style.buttonIcon}
                        dangerouslySetInnerHTML={{__html: Svgs.VIDEO_CAMERA_3}}
                    />
                    {'JOIN MEETING'}
                </a>
            );

            if (props.meeting_personal) {
                subtitle = (
                    <span>
                        {'Personal Meeting ID (PMI) : '}
                        <a
                            rel='noopener noreferrer'
                            target='_blank'
                            href={props.meeting_link}
                        >
                            {props.meeting_id}
                        </a>
                    </span>
                );
            } else {
                subtitle = (
                    <span>
                        {'Meeting ID : '}
                        <a
                            rel='noopener noreferrer'
                            target='_blank'
                            href={props.meeting_link}
                        >
                            {props.meeting_id}
                        </a>
                    </span>
                );
            }
        } else if (props.meeting_status === 'ENDED') {
            preText = 'I have ended the meeting';
            if (this.props.fromBot) {
                preText = `${this.props.creatorName} has ended the meeting`;
            }

            if (props.meeting_personal) {
                subtitle = 'Personal Meeting ID (PMI) : ' + props.meeting_id;
            } else {
                subtitle = 'Meeting ID : ' + props.meeting_id;
            }

            const startDate = new Date(post.create_at);
            const start = formatDate(startDate);
            const length = Math.ceil((new Date(post.update_at) - startDate) / 1000 / 60);

            content = (
                <div>
                    <h2 style={style.summary}>
                        {'Meeting Summary'}
                    </h2>
                    <span style={style.summaryItem}>{'Date: ' + start}</span>
                    <br/>
                    <span style={style.summaryItem}>{'Meeting Length: ' + length + ' minute(s)'}</span>
                </div>
            );
        } else if (props.meeting_status === 'RECENTLY_CREATED') {
            preText = `${this.props.creatorName} already created a call with a different provider recently`;
            if (props.meeting_provider) {
                preText = `${this.props.creatorName} already created a ${props.meeting_provider} call recently`;
            }

            subtitle = 'What do you want to do?';
            content = (
                <div>
                    <div>
                        <a
                            className='btn btn-lg btn-primary'
                            style={style.button}
                            rel='noopener noreferrer'
                            onClick={() => this.props.actions.startMeeting(this.props.currentChannelId, true)}
                        >
                            {'CREATE NEW MEETING'}
                        </a>
                    </div>
                    <div>
                        <a
                            className='btn btn-lg btn-primary'
                            style={style.button}
                            rel='noopener noreferrer'
                            target='_blank'
                            href={props.meeting_link}
                        >
                            <i
                                style={style.buttonIcon}
                                dangerouslySetInnerHTML={{__html: Svgs.VIDEO_CAMERA_3}}
                            />
                            {'JOIN EXISTING MEETING'}
                        </a>
                    </div>
                </div>
            );
        }

        let title = 'Zoom Meeting';
        if (props.meeting_topic) {
            title = props.meeting_topic;
        }

        return (
            <div className='attachment attachment--pretext'>
                <div className='attachment__thumb-pretext'>
                    {preText}
                </div>
                <div className='attachment__content'>
                    <div className='clearfix attachment__container'>
                        <h5
                            className='mt-1'
                            style={style.title}
                        >
                            {title}
                        </h5>
                        {subtitle}
                        <div>
                            <div style={style.body}>
                                {content}
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        );
    }
}

const getStyle = makeStyleFromTheme((theme) => {
    return {
        body: {
            overflowX: 'auto',
            overflowY: 'hidden',
            paddingRight: '5px',
            width: '100%',
        },
        title: {
            fontWeight: '600',
        },
        button: {
            fontFamily: 'Open Sans',
            fontSize: '12px',
            fontWeight: 'bold',
            letterSpacing: '1px',
            lineHeight: '19px',
            marginTop: '12px',
            borderRadius: '4px',
            color: theme.buttonColor,
        },
        buttonIcon: {
            paddingRight: '8px',
            fill: theme.buttonColor,
        },
        summary: {
            fontFamily: 'Open Sans',
            fontSize: '14px',
            fontWeight: '600',
            lineHeight: '26px',
            margin: '0',
            padding: '14px 0 0 0',
        },
        summaryItem: {
            fontFamily: 'Open Sans',
            fontSize: '14px',
            lineHeight: '26px',
        },
    };
});
