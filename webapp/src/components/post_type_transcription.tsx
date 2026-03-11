// Copyright (c) 2019-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {Post} from '@mattermost/types/posts';

import {AISummaryButton} from './ai_summary_button';

type Props = {
    post: Post;
};

export const PostTypeTranscription = (props: Props) => {
    return (
        <div data-testid={'zoom-post-transcription-body'}>
            {props.post.message}
            <AISummaryButton
                post={props.post}
                messageId='create-meeting-summary'
                defaultMessage='Create meeting summary'
            />
        </div>
    );
};
