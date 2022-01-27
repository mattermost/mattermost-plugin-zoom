// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.
// <reference path="../support/index.d.ts" />

// ***************************************************************
// - [#] indicates a test step (e.g. # Go to a page)
// - [*] indicates an assertion (e.g. * Check the title)
// - Use element ID when selecting an element. Create one if none.
// ***************************************************************

/**
 * Note : This test requires the demo plugin tar file under fixtures folder.
 * Download :
 * make dist latest master and copy to ./e2e/cypress/fixtures/com.mattermost.demo-plugin-0.9.0.tar.gz
 */

const defaultPluginConfig = {
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
    let settingsWithGenerated;
    let testTeam;

    const adminUsername = Cypress.env('adminUsername');
    const pluginId = 'zoom';
    const botUsername = 'zoom';

    before(() => {
        cy.apiAdminLogin();
        cy.apiCreateOrGetTeam('test').then(({team}) => {
            testTeam = team;
        });
    })

    beforeEach(() => {
        cy.apiAdminLogin();
        cy.apiRemoveAllPostsInDirectChannel(adminUsername, botUsername);

        cy.apiUpdateConfig({
            PluginSettings: {
                Plugins: {
                    [pluginId]: defaultPluginConfig,
                },
            },
        });

        cy.apiDisablePluginById(pluginId);
        cy.apiEnablePluginById(pluginId);

        cy.apiGetConfig().then(({config}) => {
            const pluginSettings = config.PluginSettings.Plugins[pluginId];
            settingsWithGenerated = {
                ...defaultPluginConfig,
                encryptionkey: pluginSettings.encryptionkey,
                webhooksecret: pluginSettings.webhooksecret,
            }

            // Check if default config values were set
            expect(pluginSettings.encryptionkey).to.not.equal('');
            expect(pluginSettings.webhooksecret).to.not.equal('');

            // Make sure we're starting with a clean config otherwise, for each test
            expect(pluginSettings).to.deep.equal(settingsWithGenerated);
        });

        cy.visit(`/${testTeam.name}/messages/@${botUsername}`);
    });

    it('Zoom setup flow, with Zoom cloud', () => {
        cy.get('#post_textbox').clear().type('/zoom get-started');
        cy.get('#post_textbox').type(' ');
        cy.get('#post_textbox').type('{enter}');

        let steps = [
            ['Continue', '', 'Welcome to Zoom for Mattermost!'],
            ['No', '', 'Are you using a self-hosted private cloud or on-prem Zoom server?'],
            ['Continue', '', 'Go to https://marketplace.zoom.us'],
            ['Continue', '', 'Choose Account-level app as the app type.'],
            ['Enter Client ID and Client secret', '', 'In the App Credentials tab, note the values for Client ID and Client secret'],
        ]
        steps.forEach(handleClickStep);

        // Enter credentials into interactive dialog
        cy.get('input#client_id').type('the_client_id');
        cy.get('input#client_secret').type('the_client_secret');
        cy.get('button#interactiveDialogSubmit').click();

        steps = [
            ['Continue', 'Set OAuth redirect URL in Zoom'],
            ['Continue', 'Configure webhook in Zoom'],
            ['Continue', 'Select webhook events'],
            ['Continue', 'Select OAuth scopes'],
            ['', "You're finished setting up the plugin!"],
        ]
        steps.forEach(handleClickStep);

        cy.apiGetConfig().then(({config}) => {
            const pluginSettings = config.PluginSettings.Plugins[pluginId];

            expect(pluginSettings).to.deep.equal({
                ...settingsWithGenerated,
                oauthclientid: 'the_client_id',
                oauthclientsecret: 'the_client_secret',
            });
        });
    });

    it('Zoom setup flow, with Zoom self-hosted', () => {
        cy.get('#post_textbox').clear().type('/zoom get-started');
        cy.get('#post_textbox').type(' ');
        cy.get('#post_textbox').type('{enter}');

        let steps = [
            ['Continue', '', 'Welcome to Zoom for Mattermost!'],
            ['Yes', '', 'Are you using a self-hosted private cloud or on-prem Zoom server?'],
        ]
        steps.forEach(handleClickStep);

        // Enter self-hosted Zoom URL into interactive dialog
        cy.get('input#ZoomURL').type('https://the_zoom_url.com');
        cy.get('input#ZoomAPIURL').type('https://the_zoom_api_url.com');
        cy.get('button#interactiveDialogSubmit').click();

        steps = [
            ['Continue', '', 'Go to https://marketplace.zoom.us'],
            ['Continue', '', 'Choose Account-level app as the app type.'],
            ['Enter Client ID and Client secret', '', 'In the App Credentials tab, note the values for Client ID and Client secret'],
        ];
        steps.forEach(handleClickStep);

        // Enter credentials into interactive dialog
        cy.get('input#client_id').type('the_client_id');
        cy.get('input#client_secret').type('the_client_secret');
        cy.get('button#interactiveDialogSubmit').click();

        steps = [
            ['Continue', 'Set OAuth redirect URL in Zoom'],
            ['Continue', 'Configure webhook in Zoom'],
            ['Continue', 'Select webhook events'],
            ['Continue', 'Select OAuth scopes'],
            ['', "You're finished setting up the plugin!"],
        ];
        steps.forEach(handleClickStep);

        cy.apiGetConfig().then(({config}) => {
            const pluginSettings = config.PluginSettings.Plugins[pluginId];

            expect(pluginSettings).to.deep.equal({
                ...settingsWithGenerated,
                oauthclientid: 'the_client_id',
                oauthclientsecret: 'the_client_secret',
                zoomurl: 'https://the_zoom_url.com',
                zoomapiurl: 'https://the_zoom_api_url.com',
            });
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
