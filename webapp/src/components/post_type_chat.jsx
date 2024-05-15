import React from 'react';
import PropTypes from 'prop-types';
import {FormattedMessage} from 'react-intl';
import {useSelector} from 'react-redux';

import styled from 'styled-components';

import IconAI from 'src/components/ai_icon';

const useAIAvailable = () => {
    //@ts-ignore plugins state is a thing
    return useSelector((state) => Boolean(state.plugins?.plugins?.[aiPluginID]));
};

const aiPluginID = 'mattermost-ai';

const useCallsPostButtonClicked = () => {
    return useSelector((state) => {
        //@ts-ignore plugins state is a thing
        return state['plugins-' + aiPluginID]?.callsPostButtonClickedTranscription;
    });
};

const CreateMeetingSummaryButton = styled.button`
	display: flex;
	border: none;
	height: 24px;
	padding: 4px 10px;
	margin-top: 8px;
	margin-bottom: 8px;
	align-items: center;
	justify-content: center;
	gap: 6px;
	border-radius: 4px;
	background: rgba(var(--center-channel-color-rgb), 0.08);
    color: rgba(var(--center-channel-color-rgb), 0.64);
	font-size: 12px;
	font-weight: 600;
	line-height: 16px;

	&:hover {
		background: rgba(var(--center-channel-color-rgb), 0.12);
        color: rgba(var(--center-channel-color-rgb), 0.72);
	}

	&:active {
		background: rgba(var(--button-bg-rgb), 0.08);
		color: var(--button-bg);
	}
`;

export const PostTypeChat = (props) => {
    const aiAvailable = useAIAvailable();
    const callsPostButtonClicked = useCallsPostButtonClicked();

    const createMeetingSummary = () => {
        callsPostButtonClicked?.(props.post);
    };

    const markdownMessage = props.post.message;

    const renderPostWithMarkdown = (msg) => {
        const {formatText, messageHtmlToComponent} = window.PostUtils;

        return messageHtmlToComponent(
            formatText(msg, {}),
            false,
        );
    };

    return (
        <div data-testid={'zoom-post-transcription-body'}>
            {renderPostWithMarkdown(markdownMessage)}
            {aiAvailable && callsPostButtonClicked &&
            <CreateMeetingSummaryButton
                onClick={createMeetingSummary}
            >
                <IconAI/>
                <FormattedMessage
                    id='summarize-chat-history'
                    defaultMessage={'Summarize chat history'}
                />
            </CreateMeetingSummaryButton>
            }
        </div>
    );
};

PostTypeChat.propTypes = {
    post: PropTypes.object.isRequired,
};
