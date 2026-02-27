// Copyright (c) 2019-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {FormattedMessage} from 'react-intl';
import {useSelector} from 'react-redux';

import type {Post} from '@mattermost/types/posts';

import styled from 'styled-components';

import IconAI from 'src/components/ai_icon';

const aiPluginID = 'mattermost-ai';

export const useAIAvailable = () => {
    return useSelector((state: any) => Boolean(state.plugins?.plugins?.[aiPluginID]));
};

export const useCallsPostButtonClicked = () => {
    return useSelector((state: any) => {
        const aiPluginState = state['plugins-' + aiPluginID];
        const handler = aiPluginState?.callsPostButtonClickedTranscription;
        if (typeof handler === 'function') {
            return handler;
        }
        return null;
    });
};

const SummaryButton = styled.button`
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
    messageId: string;
    defaultMessage: string;
};

export const AISummaryButton = ({post, messageId, defaultMessage}: Props) => {
    const aiAvailable = useAIAvailable();
    const callsPostButtonClicked = useCallsPostButtonClicked();

    if (!aiAvailable || !callsPostButtonClicked) {
        return null;
    }

    const handleClick = () => {
        callsPostButtonClicked(post);
    };

    return (
        <SummaryButton onClick={handleClick}>
            <IconAI/>
            <FormattedMessage
                id={messageId}
                defaultMessage={defaultMessage}
            />
        </SummaryButton>
    );
};
