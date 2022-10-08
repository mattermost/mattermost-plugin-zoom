// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

import React from 'react';
import PropTypes from 'prop-types';
import {FormattedMessage} from 'react-intl';
import {makeStyleFromTheme} from 'mattermost-redux/utils/theme_utils';

import {Svgs} from '../../constants';

export default class ChannelHeaderIcon extends React.PureComponent {
    static propTypes = {
        useSVG: PropTypes.bool.isRequired,
    }
    render() {
        const style = getStyle();

        let icon = (ariaLabel) => (
            <span aria-label={ariaLabel}>
                <i className='icon icon-brand-zoom'/>
            </span>
        );
        if (this.props.useSVG) {
            icon = (ariaLabel) => (
                <span
                    aria-label={ariaLabel}
                    style={style.iconStyle}
                    dangerouslySetInnerHTML={{__html: Svgs.VIDEO_CAMERA}}
                />
            );
        }
        return (
            <FormattedMessage
                id='zoom.camera.ariaLabel'
                defaultMessage='zoom camera icon'
            >
                {(ariaLabel) => icon(ariaLabel)}
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
