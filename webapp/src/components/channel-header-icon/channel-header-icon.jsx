// Copyright (c) 2019-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import PropTypes from 'prop-types';
import {useIntl} from 'react-intl';
import {makeStyleFromTheme} from 'mattermost-redux/utils/theme_utils';

import {Svgs} from '../../constants';

function ChannelHeaderIcon({useSVG}) {
    const intl = useIntl();
    const style = getStyle();
    const ariaLabel = intl.formatMessage({
        id: 'zoom.camera.ariaLabel',
        defaultMessage: 'Zoom camera icon',
    });

    if (useSVG) {
        return (
            <span
                aria-label={ariaLabel}
                style={style.iconStyle}
                dangerouslySetInnerHTML={{__html: Svgs.VIDEO_CAMERA}}
            />
        );
    }

    return (
        <span aria-label={ariaLabel}>
            <i className='icon icon-brand-zoom'/>
        </span>
    );
}

ChannelHeaderIcon.propTypes = {
    useSVG: PropTypes.bool.isRequired,
};

export default ChannelHeaderIcon;

const getStyle = makeStyleFromTheme(() => {
    return {
        iconStyle: {
            position: 'relative',
            top: '-1px',
        },
    };
});
