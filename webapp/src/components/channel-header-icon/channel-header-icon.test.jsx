// Copyright (c) 2019-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {render, screen} from '@testing-library/react';
import {IntlProvider} from 'react-intl';

import ChannelHeaderIcon from './channel-header-icon';

const renderWithIntl = (component) => {
    return render(
        <IntlProvider
            locale='en'
            messages={{}}
        >
            {component}
        </IntlProvider>,
    );
};

describe('ChannelHeaderIcon', () => {
    test('renders with icon class when useSVG is false', () => {
        renderWithIntl(<ChannelHeaderIcon useSVG={false}/>);
        expect(screen.getByLabelText('Zoom camera icon')).toBeInTheDocument();
        expect(document.querySelector('.icon-brand-zoom')).toBeInTheDocument();
    });

    test('renders with SVG when useSVG is true', () => {
        renderWithIntl(<ChannelHeaderIcon useSVG={true}/>);
        expect(screen.getByLabelText('Zoom camera icon')).toBeInTheDocument();
    });
});
