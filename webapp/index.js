// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

const React = window.react;

import Icon from './components/icon.jsx';
import PostTypeZoom from './components/post_type_zoom';
import {startMeeting} from './actions'

class PluginClass {
    initialize(registry, store) {
        registry.registerChannelHeaderButtonAction(
            <Icon/>,
            (channel) => {
                startMeeting(channel.id)(store.dispatch, store.getState);
            },
            'Start Zoom Meeting'
        );
        registry.registerPostTypeComponent('custom_zoom', PostTypeZoom)
    }
}

global.window.plugins['zoom'] = new PluginClass();
