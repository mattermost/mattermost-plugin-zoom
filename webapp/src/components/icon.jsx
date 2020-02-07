// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

import React from 'react';
import {FormattedMessage} from 'react-intl';
import {makeStyleFromTheme} from 'mattermost-redux/utils/theme_utils';

import {Svgs} from '../constants';

export default class Icon extends React.PureComponent {
    render() {
        const style = getStyle();

        return (
            <FormattedMessage
                id='zoom.camera.ariaLabel'
                defaultMessage='camera icon'
            >
                {(ariaLabel) => (
                    <span
                        style={style.iconStyle}
                        aria-label={ariaLabel}
                        dangerouslySetInnerHTML={{__html: Svgs.VIDEO_CAMERA}}
                    />
                )}
            </FormattedMessage>
        );
    }
}

const getStyle = makeStyleFromTheme(() => {
    return {
        iconStyle: {
            position: 'relative',
            top: '-1px',
        },
    };
});
