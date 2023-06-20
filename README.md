# Mattermost Zoom Plugin

[![Build Status](https://img.shields.io/circleci/project/github/mattermost/mattermost-plugin-zoom/master)](https://circleci.com/gh/mattermost/mattermost-plugin-zoom)
[![Code Coverage](https://img.shields.io/codecov/c/github/mattermost/mattermost-plugin-zoom/master)](https://codecov.io/gh/mattermost/mattermost-plugin-zoom)
[![Release](https://img.shields.io/github/v/release/mattermost/mattermost-plugin-zoom)](https://github.com/mattermost/mattermost-plugin-zoom/releases/latest)
[![HW](https://img.shields.io/github/issues/mattermost/mattermost-plugin-zoom/Up%20For%20Grabs?color=dark%20green&label=Help%20Wanted)](https://github.com/mattermost/mattermost-plugin-zoom/issues?q=is%3Aissue+is%3Aopen+sort%3Aupdated-desc+label%3A%22Up+For+Grabs%22+label%3A%22Help+Wanted%22)

**Maintainer:** [@mickmister](https://github.com/mickmister)
**Co-Maintainer:** [@trilopin](https://github.com/trilopin)

The Mattermost/Zoom integration allows team members to initiate a Zoom meeting with a single click. All participants in a channel can easily join the Zoom meeting and the shared link is updated when the meeting is over.

**Important**: Only Zoom users associated with the Zoom Account that created the Zoom App will be able to use the plugin. You can add these users from the **Manage Users** section in the Zoom Account settings.

![image](https://github.com/mattermost/mattermost-plugin-zoom/assets/74422101/7ce3981e-bcd2-4446-bc20-b9a6494dcc3d)

## Admin guide

### Installation

[Install Zoom plugin](https://github.com/mattermost/mattermost-plugin-zoom/blob/release_v1.5.1-cloud/docs/installation/install-zoom-plugin.md)

[Configure Zoom plugin](https://github.com/mattermost/mattermost-plugin-zoom/blob/release_v1.5.1-cloud/docs/installation/zoom-configuration/README.md)

  [Zoom setup (User Level App)](https://mattermost.gitbook.io/plugin-zoom/v/release_v1.5.1-cloud/installation/zoom-configuration/zoom-setup-user-level-app)

  [Zoom setup (Account Level App)](https://mattermost.gitbook.io/plugin-zoom/v/release_v1.5.1-cloud/installation/zoom-configuration/zoom-setup-oauth)

  [Webhook configuration](https://mattermost.gitbook.io/plugin-zoom/v/release_v1.5.1-cloud/installation/zoom-configuration/webhook-configuration)

[Configure Mattermost for the Zoom plugin](https://mattermost.gitbook.io/plugin-zoom/v/release_v1.5.1-cloud/installation/mattermost-setup)

## User guide

[Connect your account](https://github.com/mattermost/mattermost-plugin-zoom/blob/release_v1.5.1-cloud/docs/usage/connect-your-account.md)

[Start meetings](https://github.com/mattermost/mattermost-plugin-zoom/blob/release_v1.5.1-cloud/docs/usage/start-meetings.md)

## Development

This plugin contains both a server and web app portion. 

Use `make dist` to build distributions of the plugin that you can upload to a Mattermost server for testing.

Use `make check-style` to check the style for the whole plugin.

### Server

Inside the `/server` directory, you will find the Go files that make up the server-side of the plugin. Within there, build the plugin like you would any other Go application.

### Web App

Inside the `/webapp` directory, you will find the JS and React files that make up the client-side of the plugin. Within there, modify files and components as necessary. Test your syntax by running `npm run build`.

Read our documentation about the [Developer Workflow](https://developers.mattermost.com/extend/plugins/developer-workflow/) and [Developer Setup](https://developers.mattermost.com/extend/plugins/developer-setup/) for more information about developing and extending plugins.

## Help and Support

For Mattermost customers - please open a [support case](https://mattermost.zendesk.com/hc/en-us/requests/new) to ensure your issue is tracked properly.

For Questions, Suggestions and Help - please find us on our forum at [https://forum.mattermost.org/c/plugins](https://forum.mattermost.org/c/plugins)​

Alternatively, join our pubic Mattermost server and join the Integrations and Apps channel here: [https://community-daily.mattermost.com/core/channels/integrations](https://community-daily.mattermost.com/core/channels/integrations)​

To Contribute to the Mattermost project see [https://www.mattermost.org/contribute-to-mattermost/](https://www.mattermost.org/contribute-to-mattermost/)
