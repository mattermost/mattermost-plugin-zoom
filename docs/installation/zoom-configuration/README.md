---
description: >-
  You'll need to choose which authentication method you want your users to
  connect their Zoom accounts
---

# Zoom Configuration

Zoom supports two authentication methods for users to connect Mattermost and Zoom: OAuth or JWT/Password based authentication.

1. [Oauth](zoom-setup-oauth.md)
   * Users need to connect their Zoom account with their Mattermost account before they can use the integration. When they try to create a meeting for the first time, they'll receive a message to connect their account. They'l need to select **Approve** on a pop-up web page, in order to begin the meeting.
   * Users can connect their Mattermost/Zoom accounts even if their email addresses do not match.
2. [JWT/Password](zoom-setup-jwt.md)
   * Users don't have to connect their account to use the integration which makes it easy to get started.
   * Uses JWT to pass security tokens. This may not be sufficiently secure for some customers.
