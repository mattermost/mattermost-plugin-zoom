import React from 'react';
import {useSelector} from 'react-redux';

import {getSiteURL} from '../../selectors';

const appBarIconPath = '/plugins/zoom/public/app-bar-icon.png';

export default function AppBarIcon() {
    const siteURL = useSelector(getSiteURL);
    const iconURL = siteURL + appBarIconPath;

    return (
        <img src={iconURL}/>
    );
}
