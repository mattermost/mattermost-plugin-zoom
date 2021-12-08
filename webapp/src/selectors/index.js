import {getConfig} from 'mattermost-redux/selectors/entities/general';

import {id} from '../manifest';

export function getPluginURL(state) {
    const config = getConfig(state);
    return `${config.SiteURL}/plugins/${id}`;
}
