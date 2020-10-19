---
description: >-
 Choose which authentication method you want your users to use to connect their Zoom accounts
---

# Zoom Configuration

Zoom supports two authentication methods for users to connect Mattermost and Zoom: OAuth or JWT/Password based authentication.

1. [Oauth](zoom-setup-oauth.md)
   * Users need to connect their Zoom account with their Mattermost account before they can use the integration. When they try to create a meeting for the first time, they'll receive a message to connect their account. They'll need to select **Approve** on a pop-up web page, in order to begin the meeting.
   * Users can connect their Mattermost/Zoom accounts even if their email addresses do not match.
2. [JWT/Password](zoom-setup-jwt.md)
   * Users don't have to connect their account to use the integration which makes it easy to get started.
   * Uses JWT to pass security tokens. This may not be sufficiently secure for some customers.
   * The users must have the same email registered both in Zoom and Mattermost.

## Upgrading From a Previous Version

If you've been using Zoom prior to v1.4, you likely have a legacy webhook-type app configured in Zoom.

Legacy webhook apps are no longer supported by Zoom or Mattermost and are not compatible with Zoom plugin v1.4. You may experience issues with the meeting status message information not being updated when a meeting ends. This is because the webhook endpoint expects a JSON format request and newer webhooks use different formats.

From Zoom v1.4, you can configure and associate your webhooks with an app you've already created. First, remove the previous webhook app. Then follow the steps provided [here](https://mattermost.gitbook.io/plugin-zoom/installation/zoom-configuration/webhook-configuration) to configure the new webhook.

