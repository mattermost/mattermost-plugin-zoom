# Mattermost Zoom Plugin 

[![Build Status](https://img.shields.io/circleci/project/github/mattermost/mattermost-plugin-zoom/master)](https://circleci.com/gh/mattermost/mattermost-plugin-zoom)
[![Code Coverage](https://img.shields.io/codecov/c/github/mattermost/mattermost-plugin-zoom/master)](https://codecov.io/gh/mattermost/mattermost-plugin-zoom)
[![Release](https://img.shields.io/github/v/release/mattermost/mattermost-plugin-zoom)](https://github.com/mattermost/mattermost-plugin-zoom/releases/latest)
[![HW](https://img.shields.io/github/issues/mattermost/mattermost-plugin-zoom/Up%20For%20Grabs?color=dark%20green&label=Help%20Wanted)](https://github.com/mattermost/mattermost-plugin-zoom/issues?q=is%3Aissue+is%3Aopen+sort%3Aupdated-desc+label%3A%22Up+For+Grabs%22+label%3A%22Help+Wanted%22)


**Maintainer:** [@larkox](https://github.com/larkox)
**Co-Maintainer:** [@mickmister](https://github.com/mickmister)

Start and join voice calls, video calls and use screen sharing with your team members via Zoom.

## Usage & Setup Guide
### Commands
* **`/zoom start`** &mdash; Start a zoom meeting.
* **`/zoom disconnect`** &mdash; Disconnect from zoom (if OAuth is enabled).
* **`/zoom help`** &mdash; Display help message.

Check our [GitBook](https://app.gitbook.com/@mattermost/s/plugin-zoom/) for further documentation about this plugin.

## Development

This plugin contains both a server and web app portion.

Use `make dist` to build distributions of the plugin that you can upload to a Mattermost server for testing.

Use `make check-style` to check the style for the whole plugin.

### Server

Inside the `/server` directory, you will find the Go files that make up the server-side of the plugin. Within there, build the plugin like you would any other Go application.

### Web App

Inside the `/webapp` directory, you will find the JS and React files that make up the client-side of the plugin. Within there, modify files and components as necessary. Test your syntax by running `npm run build`.
