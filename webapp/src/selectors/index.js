import {getConfig} from 'mattermost-redux/selectors/entities/general';

export function getSiteURL(state) {
    const config = getConfig(state);
    return config.SiteURL;
}
