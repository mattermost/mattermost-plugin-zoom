// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

import React from 'react';

import {getConfig} from 'mattermost-redux/selectors/entities/general';

import {id as pluginId} from './manifest';

import ChannelHeaderIcon from './components/channel-header-icon';
import AppBarIcon from './components/app-bar-icon/app-bar-icon';
import PostTypeZoom from './components/post_type_zoom';
import {startMeeting} from './actions';
import Client from './client';

class Plugin {
    // eslint-disable-next-line no-unused-vars
    initialize(registry, store) {
        registry.registerChannelHeaderButtonAction(
            <ChannelHeaderIcon/>,
            (channel) => {
                startMeeting(channel.id)(store.dispatch, store.getState);
            },
            'Start Zoom Meeting',
            'Start Zoom Meeting',
        );

        if ('registerAppBarComponent' in registry) {
            registry.registerAppBarComponent(
                <AppBarIcon/>,
                (channel) => {
                    startMeeting(channel.id)(store.dispatch, store.getState);
                },
                'Start Zoom Meeting',
            );
        }

        registry.registerPostTypeComponent('custom_zoom', PostTypeZoom);
        Client.setServerRoute(getServerRoute(store.getState()));
    }
}

window.registerPlugin(pluginId, new Plugin());

const getServerRoute = (state) => {
    const config = getConfig(state);

    let basePath = '';
    if (config && config.SiteURL) {
        basePath = new URL(config.SiteURL).pathname;

        if (basePath && basePath[basePath.length - 1] === '/') {
            basePath = basePath.substr(0, basePath.length - 1);
        }
    }

    return basePath;
};
