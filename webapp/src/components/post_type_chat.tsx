// Copyright (c) 2019-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {Post} from '@mattermost/types/posts';

import {AISummaryButton} from './ai_summary_button';

type Props = {
    post: Post;
};

const renderPostWithMarkdown = (msg: string) => {
    const postUtils = (window as any).PostUtils;
    if (!postUtils?.formatText || !postUtils?.messageHtmlToComponent) {
        return <span>{msg}</span>;
    }

    return postUtils.messageHtmlToComponent(
        postUtils.formatText(msg, {}),
        false,
    );
};

export const PostTypeChat = (props: Props) => {
    return (
        <div data-testid={'zoom-post-chat-body'}>
            {renderPostWithMarkdown(props.post.message)}
            <AISummaryButton
                post={props.post}
                messageId='summarize-chat-history'
                defaultMessage='Summarize chat history'
            />
        </div>
    );
};
