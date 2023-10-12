// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

import React from 'react';

import {id as pluginId} from './manifest';

import ChannelHeaderIcon from './components/channel-header-icon';
import PostTypeZoom from './components/post_type_zoom';
import {startMeeting} from './actions';
import Client from './client';
import {getPluginURL, getServerRoute} from './selectors';

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

        if (registry.registerAppBarComponent) {
            const iconURL = getPluginURL(store.getState()) + '/public/app-bar-icon.png';
            registry.registerAppBarComponent(
                iconURL,
                async (channel) => {
                    if (channel) {
                        startMeeting(channel.id, '')(store.dispatch, store.getState);
                    } else {
                        const teamId = store.getState().entities.teams.currentTeamId;
                        const threadId = store.getState().views.threads.selectedThreadIdInTeam[teamId];
                        const baseURL = store.getState().entities.general.config.SiteURL;
                        const channelId = await Client.getChannelId(baseURL, threadId);
                        startMeeting(channelId, threadId)(store.dispatch, store.getState);
                    }
                },
                'Start Zoom Meeting',
            );
        }

        registry.registerPostTypeComponent('custom_zoom', PostTypeZoom);
        Client.setServerRoute(getServerRoute(store.getState()));
    }
}

window.registerPlugin(pluginId, new Plugin());
