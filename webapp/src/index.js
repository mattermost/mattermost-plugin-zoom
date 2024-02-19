// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

import React from 'react';

import {id as pluginId} from './manifest';

import ChannelHeaderIcon from './components/channel-header-icon';
import PostTypeZoom from './components/post_type_zoom';
import {openScheduleMeetingModal, startMeeting} from './actions';
import Client from './client';
import {getPluginURL, getServerRoute} from './selectors';
import Reducer from './reducers'
import ScheduleMeetingModal from './components/schedule_meeting/schedule_meeting_modal'

class Plugin {
    // eslint-disable-next-line no-unused-vars
    initialize(registry, store) {
        registry.registerReducer(Reducer);
        registry.registerChannelHeaderButtonAction(
            <ChannelHeaderIcon/>,
            (channel) => {
                startMeeting(channel.id)(store.dispatch, store.getState);
            },
            'Start Zoom Meeting',
            'Start Zoom Meeting',
        );

        registry.registerRootComponent(ScheduleMeetingModal);

        if (registry.registerAppBarComponent) {
            const iconURL = getPluginURL(store.getState()) + '/public/app-bar-icon.png';
            registry.registerAppBarComponent(
                iconURL,
                async (channel) => {
                    if (channel) {
                        openScheduleMeetingModal(channel.id)(store.dispatch);
                        // startMeeting(channel.id, '')(store.dispatch, store.getState);
                    } else {
                        const state = store.getState();
                        const teamId = state?.entities.teams.currentTeamId;
                        const threadId = state?.views.threads.selectedThreadIdInTeam[teamId];
                        const baseURL = state?.entities.general.config.SiteURL;
                        const channelId = await Client.getChannelIdForThread(baseURL, threadId);
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
