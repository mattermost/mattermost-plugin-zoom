// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

import React from 'react';
import {FormattedMessage} from 'react-intl';
import {makeStyleFromTheme} from 'mattermost-redux/utils/theme_utils';

export default class Icon extends React.PureComponent {
    render() {
        const style = getStyle();

        return (
            <FormattedMessage
                id='zoom.camera.ariaLabel'
                defaultMessage='zoom camera icon'
            >
                {(ariaLabel) => (
                    <span
                        style={style.iconStyle}
                        aria-label={ariaLabel}
                    >
                        <i className='icon icon-brand-zoom'/>
                    </span>
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
