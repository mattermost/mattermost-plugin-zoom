import React from 'react';
import {FormattedMessage} from 'react-intl';
import {useSelector} from 'react-redux';

import type {Post} from 'mattermost-redux/types/posts';
import type {GlobalState} from 'mattermost-redux/types/store';

import styled from 'styled-components';

import IconAI from 'src/components/ai_icon';

const aiPluginID = 'mattermost-ai';

const useAIAvailable = () => {
    return useSelector((state: any) => Boolean(state.plugins?.plugins?.[aiPluginID]));
};

const useCallsPostButtonClicked = () => {
    return useSelector((state: GlobalState) => {
        type StateWithAiPluginState = {
            'plugins-mattermost-ai'?: {callsPostButtonClickedTranscription: (post: Post) => void};
        }
        const stateTyped: StateWithAiPluginState = state as StateWithAiPluginState;
        const aiPluginState = stateTyped['plugins-mattermost-ai'];
        return aiPluginState?.callsPostButtonClickedTranscription;
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

type Props = {
    post: Post;
};

export const PostTypeTranscription = (props: Props) => {
    const aiAvailable = useAIAvailable();
    const callsPostButtonClicked = useCallsPostButtonClicked();

    const createMeetingSummary = () => {
        callsPostButtonClicked?.(props.post);
    };

    const msg = props.post.message;

    return (
        <div data-testid={'zoom-post-transcription-body'}>
            {msg}
            {aiAvailable && callsPostButtonClicked && (
                <CreateMeetingSummaryButton
                    onClick={createMeetingSummary}
                >
                    <IconAI/>
                    <FormattedMessage
                        id='create-meeting-summary'
                        defaultMessage={'Create meeting summary?'}
                    />
                </CreateMeetingSummaryButton>
            )}
        </div>
    );
};
