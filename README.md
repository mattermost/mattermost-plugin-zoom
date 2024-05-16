# Mattermost Zoom Plugin

[![Build Status](https://img.shields.io/circleci/project/github/mattermost/mattermost-plugin-zoom/master)](https://circleci.com/gh/mattermost/mattermost-plugin-zoom)
[![Code Coverage](https://img.shields.io/codecov/c/github/mattermost/mattermost-plugin-zoom/master)](https://codecov.io/gh/mattermost/mattermost-plugin-zoom)
[![Release](https://img.shields.io/github/v/release/mattermost/mattermost-plugin-zoom)](https://github.com/mattermost/mattermost-plugin-zoom/releases/latest)
[![HW](https://img.shields.io/github/issues/mattermost/mattermost-plugin-zoom/Up%20For%20Grabs?color=dark%20green&label=Help%20Wanted)](https://github.com/mattermost/mattermost-plugin-zoom/issues?q=is%3Aissue+is%3Aopen+sort%3Aupdated-desc+label%3A%22Up+For+Grabs%22+label%3A%22Help+Wanted%22)

**Maintainer:** [@mickmister](https://github.com/mickmister)
**Co-Maintainer:** [@trilopin](https://github.com/trilopin)

The Mattermost Zoom integration allows team members to initiate a Zoom meeting with a single click. All participants in a channel can easily join the Zoom meeting and the shared link is updated when the meeting is over.

**Important**: Only Zoom users associated with the Zoom Account that created the Zoom App will be able to use the plugin. You can add these users from the **Manage Users** section in the Zoom Account settings.

![image](https://github.com/mattermost/mattermost-plugin-zoom/assets/55234496/a109df21-f3c4-4432-b42d-7ef3ae493a50)

See the [Mattermost Product Documentation](https://docs.mattermost.com/integrate/zoom-interoperability.html) for details on installing, configuring, enabling, and using this Mattermost integration.

## Development

This plugin contains both a server and web app portion. Read our documentation about the [Developer Workflow](https://developers.mattermost.com/integrate/plugins/developer-workflow/) and [Developer Setup](https://developers.mattermost.com/integrate/plugins/developer-setup/) for more information about developing and extending plugins.

### Server

Inside the `/server` directory, you will find the Go files that make up the server-side of the plugin. Within there, build the plugin like you would any other Go application.

### Web App

Inside the `/webapp` directory, you will find the JS and React files that make up the client-side of the plugin. Within there, modify files and components as necessary. Test your syntax by running `npm run build`.

Read our documentation about the [Developer Workflow](https://developers.mattermost.com/extend/plugins/developer-workflow/) and [Developer Setup](https://developers.mattermost.com/extend/plugins/developer-setup/) for more information about developing and extending plugins.

### Releasing new versions

The version of a plugin is determined at compile time, automatically populating a `version` field in the [plugin manifest](plugin.json):
* If the current commit matches a tag, the version will match after stripping any leading `v`, e.g. `1.3.1`.
* Otherwise, the version will combine the nearest tag with `git rev-parse --short HEAD`, e.g. `1.3.1+d06e53e1`.
* If there is no version tag, an empty version will be combined with the short hash, e.g. `0.0.0+76081421`.

To disable this behaviour, manually populate and maintain the `version` field.

## Help and Support

For Mattermost customers - please open a [support case](https://mattermost.zendesk.com/hc/en-us/requests/new) to ensure your issue is tracked properly.

For Questions, Suggestions and Help - please find us on our forum at [https://forum.mattermost.org/c/plugins](https://forum.mattermost.org/c/plugins)​

Alternatively, join our pubic Mattermost server and join the Integrations and Apps channel here: [https://community.mattermost.com/core/channels/integrations](https://community-daily.mattermost.com/core/channels/integrations)​

To Contribute to the Mattermost project see [https://www.mattermost.org/contribute-to-mattermost/](https://mattermost.com/contribute/?redirect_source=mm-org).
