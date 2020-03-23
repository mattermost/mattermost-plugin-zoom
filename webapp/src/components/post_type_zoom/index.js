// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

import {connect} from 'react-redux';
import {bindActionCreators} from 'redux';

import {getBool} from 'mattermost-redux/selectors/entities/preferences';
import {getCurrentChannelId} from 'mattermost-redux/selectors/entities/common';

import {startMeeting} from '../../actions';

import PostTypeZoom from './post_type_zoom.jsx';

function mapStateToProps(state, ownProps) {
    return {
        ...ownProps,
        creatorName: ownProps.post.props.meeting_creator_username || 'Someone',
        useMilitaryTime: getBool(state, 'display_settings', 'use_military_time', false),
        currentChannelId: getCurrentChannelId(state),
    };
}

function mapDispatchToProps(dispatch) {
    return {
        actions: bindActionCreators({
            startMeeting,
        }, dispatch),
    };
}

export default connect(mapStateToProps, mapDispatchToProps)(PostTypeZoom);
