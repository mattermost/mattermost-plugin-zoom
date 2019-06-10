# Mattermost Zoom Plugin ![CircleCI branch](https://img.shields.io/circleci/project/github/mattermost/mattermost-plugin-zoom/master.svg)

Start and join voice calls, video calls and use screen sharing with your team members via Zoom.

Once enabled, clicking a video icon in a Mattermost channel invites team members to join a Zoom call, hosted using the credentials of the user who initiated the call.

![image](https://user-images.githubusercontent.com/13119842/58815561-d5164900-85f5-11e9-8e3d-e3b554a3e897.png)

## Usage and Configuration

Learn more about usage and configuration in the [Mattermost documentation](https://docs.mattermost.com/integrations/zoom.html).

## Development

This plugin contains both a server and web app portion.

Use `make dist` to build distributions of the plugin that you can upload to a Mattermost server for testing.

Use `make check-style` to check the style for the whole plugin.

### Server

Inside the `/server` directory, you will find the Go files that make up the server-side of the plugin. Within there, build the plugin like you would any other Go application.

### Web App

Inside the `/webapp` directory, you will find the JS and React files that make up the client-side of the plugin. Within there, modify files and components as necessary. Test your syntax by running `npm run build`.
