---
description: >-
  Choose which authentication method you want your users to use to connect their
  Zoom accounts
---

# Zoom Configuration

Zoom version 1.5 supports one authentication method for users to connect Mattermost and Zoom: OAuth.

* There are two types of OAuth Zoom apps you can create - **Account** or **User** level apps. You can use either one with this Zoom plugin depending on your organization's security and UX preferences.

  * **Account-level app**
    * Users don't need to authorize their Zoom accounts before starting their first Zoom meeting. The only requirement is that your Mattermost account uses the same email address as your Zoom account.
    * Users can't connect their Mattermost/Zoom accounts if the emails do not match.
  * **User-level app**
    * Each user needs to connect their Zoom account with their Mattermost account before they can use the integration. When you try to create a meeting for the first time, you'll receive a link which redirects to a Zoom-owned website. If you're not logged in, you'll need to log in. Once logged in, select **Approve** on the pop-up confirmation notice.
    * Users **can** connect their Mattermost/Zoom accounts **even if the emails do not match**.

## Upgrading from a previous version

If you've been using Zoom prior to v1.4, you likely have a legacy webhook-type app configured in Zoom.

Legacy webhook apps are no longer supported by Zoom or Mattermost and are not compatible with Zoom plugin v1.4 onwards. You may experience issues with the meeting status message information not being updated when a meeting ends. This is because the webhook endpoint expects a JSON format request and newer webhooks use different formats.

From Zoom v1.4, you can configure and associate your webhooks with an app you've already created. First, remove the previous webhook app. Then follow the steps provided [here](https://mattermost.gitbook.io/plugin-zoom/installation/zoom-configuration/webhook-configuration) to configure the new webhook.
