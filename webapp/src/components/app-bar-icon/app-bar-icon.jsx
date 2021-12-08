import React from 'react';
import {useSelector} from 'react-redux';

import {getPluginURL} from '../../selectors';

const appBarIconPath = '/public/app-bar-icon.png';

export default function AppBarIcon() {
    const pluginURL = useSelector(getPluginURL);
    const iconURL = pluginURL + appBarIconPath;

    return (
        <img src={iconURL}/>
    );
}
