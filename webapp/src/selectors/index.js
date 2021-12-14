import {getConfig} from 'mattermost-redux/selectors/entities/general';

import {id} from '../manifest';

export const getServerRoute = (state) => {
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

export const getPluginURL = (state) => {
    const siteURL = getServerRoute(state);
    return siteURL + '/plugins/' + id;
};
