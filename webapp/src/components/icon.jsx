// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

import React from 'react';
import {FormattedMessage} from 'react-intl';

import {Svgs} from '../constants';

export default class Icon extends React.PureComponent {
    render() {
        return (
            <FormattedMessage
                id='zoom.camera.ariaLabel'
                defaultMessage='zoom camera icon'
            >
                {(ariaLabel) => (
                    <span
                        className='icon icon--standard'
                        aria-label={ariaLabel}
                        dangerouslySetInnerHTML={{__html: Svgs.VIDEO_CAMERA}}
                    />
                )}
            </FormattedMessage>
        );
    }
}
