// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

import React from 'react';

import {id as pluginId} from './manifest';

import Icon from './components/icon.jsx';
import PostTypeZoom from './components/post_type_zoom';
import {startMeeting} from './actions';

class Plugin {
    // eslint-disable-next-line no-unused-vars
    initialize(registry, store) {
        registry.registerChannelHeaderButtonAction(
            <Icon/>,
            (channel) => {
                startMeeting(channel.id)(store.dispatch, store.getState);
            },
            'Start Zoom Meeting'
        );
        registry.registerPostTypeComponent('custom_zoom', PostTypeZoom);
    }
}

window.registerPlugin(pluginId, new Plugin());
