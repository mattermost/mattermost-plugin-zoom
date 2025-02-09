// Copyright (c) 2019-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {connect} from 'react-redux';

import {getServerVersion} from 'mattermost-redux/selectors/entities/general';
import {isMinimumServerVersion} from 'mattermost-redux/utils/helpers';

import ChannelHeaderIcon from './channel-header-icon';

function mapStateToProps(state, ownProps) {
    return {
        ...ownProps,
        useSVG: !isMinimumServerVersion(getServerVersion(state), 5, 24),
    };
}

export default connect(mapStateToProps)(ChannelHeaderIcon);
