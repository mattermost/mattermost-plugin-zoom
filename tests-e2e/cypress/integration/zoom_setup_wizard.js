// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.
// <reference path="../support/index.d.ts" />

// ***************************************************************
// - [#] indicates a test step (e.g. # Go to a page)
// - [*] indicates an assertion (e.g. * Check the title)
// - Use element ID when selecting an element. Create one if none.
// ***************************************************************

const defaultZoomConfig = {
    "accountlevelapp": false,
    "apikey": "",
    "apisecret": "",
    "enableoauth": true,
    "encryptionkey": "",
    "oauthclientid": "",
    "oauthclientsecret": "",
    "webhooksecret": "",
    "zoomapiurl": "",
    "zoomurl": ""
}

describe('Zoom setup wizard', () => {
    beforeEach(() => {
        // # Login as sysadmin
        cy.apiAdminLogin();

        cy.apiDeleteAllPostsInDirectChannel('sysadmin', 'zoom');

        cy.apiUpdateConfig({
            PluginSettings: {
                Plugins: {
                    zoom: defaultZoomConfig,
                },
            },
        });

        cy.apiDisablePluginById('zoom');
        cy.apiEnablePluginById('zoom');

        cy.visit('/test/messages/@zoom');
    });

    it.only('Zoom setup flow', () => {
        cy.apiGetConfig().then(({config}) => {
            const zoomSettings = config.PluginSettings.Plugins.zoom;
            const withGenerated = {
                ...defaultZoomConfig,
                encryptionkey: zoomSettings.encryptionkey,
                webhooksecret: zoomSettings.webhooksecret,
            }

            expect(zoomSettings).to.deep.equal(withGenerated);
        });

        cy.get('#post_textbox').clear().type('/zoom setup');
        cy.get('#post_textbox').type(' ');
        cy.get('#post_textbox').type('{enter}');

        let buttons = [
            ['Continue', '', 'Welcome to Zoom for Mattermost!'],
            ['No', '', 'Are you using a self-hosted private cloud or on-prem Zoom server?'],
            ['Continue', '', 'Go to https://marketplace.zoom.us'],
            ['Continue','', 'Choose Account-level app as the app type.'],
            ['Enter Client ID and Client secret', '', 'In the App Credentials tab, note the values for Client ID and Client secret'],
        ]

        buttons.forEach(handleClickStep);

        // Enter credentials into interactive dialog
        cy.get('input#client_id').type('the_client_id');
        cy.get('input#client_secret').type('the_client_secret');
        cy.get('button#interactiveDialogSubmit').click();

        buttons = [
            ['Continue', 'Set OAuth redirect URL in Zoom'],
            ['Continue', 'Configure webhook in Zoom'],
            ['Continue', 'Select webhook events'],
            ['Continue', 'Select OAuth scopes'],
            ['', "You're finished setting up the plugin!"],
        ]

        buttons.forEach(handleClickStep);

        cy.apiGetConfig().then(({config}) => {
            const zoomSettings = config.PluginSettings.Plugins.zoom;

            expect(zoomSettings.oauthclientid).to.equal('the_client_id');
            expect(zoomSettings.oauthclientsecret).to.equal('the_client_secret');
        });
    });
});

function handleClickStep(testCase) {
    const [buttonText, expectedTitle, expectedBody] = testCase;

    cy.getLastPostId().then((lastPostId) => {
        if (expectedTitle) {
            cy.getLastPostId().then((lastPostId) => {
                cy.get(`#post_${lastPostId} .attachment__title`).contains(expectedTitle);
            });
        }

        if (expectedBody) {
            cy.getLastPostId().then((lastPostId) => {
                cy.get(`#post_${lastPostId} .attachment__body`).contains(expectedBody);
            });
        }

        if (buttonText) {
            cy.get(`#${lastPostId}_message`).contains('button:enabled', buttonText).click();
        }
    });
}

const objectsEqual = (obj1, obj2) => {
    const keys1 = Object.keys(obj1);
    const keys2 = Object.keys(obj2);
    if (keys1.length !== keys2.length) {
        return false;
    }

    for (const key of keys1) {
        if (obj1[key] !== obj2[key]) {
            return false;
        }
    }

    return true;
}
