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

## Admin guide

### Install Zoom plugin

#### Install via Plugin Marketplace \(Recommended\)

1. In Mattermost, go to **Main Menu > Plugin Marketplace**.
2. Search for "Zoom" or manually find the plugin from the list and select **Install**.
3. After the plugin has downloaded and been installed, configure the plugin.
4. After configuring the plugin, configure Mattermost to use the plugin.

#### \(Alternative\) Install via Manual Upload

If your server doesn't have internet access, you can download the [latest plugin binary release](https://github.com/mattermost/mattermost-plugin-zoom/releases) and upload it to your server via **System Console > Plugin Management**. The binary releases on the page above, are the same as used by the Plugin Marketplace.

1. Go to **Main Menu > Plugin Marketplace** in Mattermost.
2. Search for "Zoom" or manually find the plugin from the list and select **Install**.
3. After the plugin has downloaded and been installed, configure the plugin.
4. After configuring the plugin, configure Mattermost to use the plugin.

### Configure the plugin

Zoom supports one authentication method for users to connect Mattermost and Zoom: **OAuth**.

* There are two types of OAuth Zoom Apps you can create. You can use either one with this Zoom plugin depending on your organization's security and UX preferences.  \(**Account** or **User** Level Apps\)
  * **Account-Level App**
    * Users do not need to authorize their Zoom accounts before starting their first Zoom meeting.  The only requirement is that their Mattermost account uses the same email address as their Zoom account.
    * Users cannot connect their Mattermost/Zoom accounts if their emails do not match.
  * **User Level App**
    * Each user will need to connect their Zoom account with their Mattermost account before they can use the integration. When they try to create a meeting for the first time, they'll receive a message to connect their account, and will need to select **Approve** on the pop-up confirmation notice.
    * Users **can** connect their Mattermost/Zoom accounts **even if their emails do not match**.

#### Upgrade from a previous version

If you've been using an older version of the Zoom plugin, you likely have a legacy webhook-type app configured in Zoom. Legacy webhook apps are no longer supported by Zoom or Mattermost and are not compatible with Zoom plugin v1.4. You may experience issues with the meeting status message information not being updated when a meeting ends. This is because the webhook endpoint expects a JSON format request and newer webhooks use different formats.

From Zoom v1.4, you can configure and associate your webhooks with an app you've already created. First, remove the previous webhook app. Then configure the webhook.

### Set up the Zoom plugin \(User Level App\)

You can set the **OAuth ClientID** and **OAuth Secret**, generated by Zoom, and use it to create meetings and pull user data.

**User-level Apps** require **each user** to authorize the Mattermost App to access their Zoom account individually. If you prefer to authorize its access by an admin on behalf of the whole Zoom organization you should create an Account-level app instead.

#### Create an app for Mattermost

1. Go to [https://marketplace.zoom.us/](https://marketplace.zoom.us/) and log in as an admin.
2. In the top left select **Develop** and then **Build App**.
3. On top you can choose either the **development** or **production** app. If you wish to publish the app on Zoom Marketplace then please select production. The plugin supports apps that are published in the Zoom Marketplace.
4. You can edit the name of your app from top left side by clicking on edit icon.
5. Choose **User-managed app** as the app type.

![app_type](https://github.com/mattermost/mattermost-plugin-zoom/assets/90389917/47b81f49-3699-4737-815d-5cc2fccab0c4)

6. Next you'll find your **Client ID** and **Client Secret**. Please copy this as these will be needed when you set up Mattermost to use the plugin.
2. Enter a valid **Redirect URL for OAuth** \(`https://SITEURL/plugins/zoom/oauth2/complete`\) and add the same URL under **Add Allow List**. Note that `SITEURL` should be your Mattermost server URL.

![credentials](https://github.com/mattermost/mattermost-plugin-zoom/assets/90389917/62c9c2d9-e9e4-42a4-9ca7-03e8767304e3)

#### Add user scopes to the app

Select **Scopes** and add the following scopes: **meeting:read:meeting**, **meeting:write:meeting**,**user:read:user**.

![scopes-user-level-app](https://github.com/mattermost/mattermost-plugin-zoom/assets/90389917/19b071be-01a8-4c76-8dcc-b43c281e9542)

#### Do not perform the install step

Zoom has one last option called **Install**. There is no need to perform this action. However, if you perform this action inadvertently, you'll see an error on Mattermost. This is expected.

#### Deauthorization

This plugin allows users to be deauthorized directly from Zoom, in order to comply with Zoom’s commitment to security and the protection of user data. If you **would like to publish on Zoom Marketplace**, you can set up a deauthorization URL.

1. Select **App Listing** and then select **Link & Support**.
2. Near the end of the page is a section called **Deauthorization Notification**.
3. Enter a valid **Endpoint URL** \(`https://SITEURL/plugins/zoom/deauthorization?secret=WEBHOOKSECRET`\).
   * `SITEURL` should be your Mattermost server URL.
   * `WEBHOOKSECRET` is generated during when setting up Mattermost to use the plugin.

![deauthorization](https://github.com/mattermost/mattermost-plugin-zoom/assets/90389917/94da92f8-8880-459d-9f04-d9ad44a0b661)

#### Finish setting up Mattermost server

Follow the instructions for [Mattermost Setup](../mattermost-setup.md)

### Set up Zoom plugin \(Account Level App\)

You can set the **OAuth ClientID** and **OAuth Secret**, generated by Zoom, and use it to create meetings and pull user data. Note that this requires admin permissions on your Zoom account.

**Account-level apps** require **an admin to authorize access to all users accounts within the Zoom account**. Individual users in Mattermost are verified by checking their Mattermost email and requesting their Personal Meeting ID via the Zoom API. The user's emails in Zoom and Mattermost accounts should match up. If you prefer for each end user to authorize individually, you should create a user level Zoom app instead.

#### Create an app for Mattermost

1. Go to [https://marketplace.zoom.us/](https://marketplace.zoom.us/) and log in as an admin.
2. In the top left select **Develop** and then **Build App**.
3. On top you can choose either the **development** or **production** app. If you wish to publish the app on Zoom Marketplace then please select production. The plugin supports apps that are published in the Zoom Marketplace.
4. You can edit the name of your app from top left side by clicking on edit icon.
5. Choose **Admin-managed app** as the app type.

![app_type](https://github.com/mattermost/mattermost-plugin-zoom/assets/90389917/3030b7ea-eea2-4fd1-b8f1-5b47a17efecc)

6. Next you'll find your **Client ID** and **Client Secret**. Please copy this as these will be needed when you set up Mattermost to use the plugin.
2. Enter a valid **Redirect URL for OAuth** \(`https://SITEURL/plugins/zoom/oauth2/complete`\) and add the same URL under **Add Allow List**. Note that `SITEURL` should be your Mattermost server URL.

#### Add user scopes to the app

Select **Scopes** and add the following scopes: **meeting:read:meeting:admin**, **meeting:write:meeting:admin**,**user:read:user:admin**.

![scopes-account-level-app](https://github.com/mattermost/mattermost-plugin-zoom/assets/90389917/07919a75-6b6b-4914-b009-70f6dfe5e906)

#### Do not perform the install step

Zoom has one last option called **Install**. There is no need to perform this action. However, if you perform this action inadvertently, you'll see an error on Mattermost. This is expected.

#### Deauthorization

This plugin allows users to be deauthorized directly from Zoom, in order to comply with Zoom’s commitment to security and the protection of user data. If you **would like to publish on Zoom Marketplace**, you can set up a deauthorization URL.

1. Select **App Listing** and then select **Link & Support**.
2. Near the end of the page, is a section called **Deauthorization Notification**.
3. Enter a valid **Endpoint URL** \(`https://SITEURL/plugins/zoom/deauthorization?secret=WEBHOOKSECRET`\).
   * `SITEURL` should be your Mattermost server URL.
   * `WEBHOOKSECRET` is generated during [Mattermost Setup](../mattermost-setup.md).

![deauthorization](https://github.com/mattermost/mattermost-plugin-zoom/assets/90389917/94da92f8-8880-459d-9f04-d9ad44a0b661)

#### Finish setting up Mattermost server

Follow the instructions for setting up Mattermost to use the plugin.

### Configure Webhook Events

When a Zoom meeting ends, the original link shared in the channel can be changed to indicate the meeting has ended and how long it lasted. To enable this functionality, we need to create a webhook subscription in Zoom that tells the Mattermost server every time a meeting ends. The Mattermost server then updates the original Zoom message.

1. Select **Access** in the Features.
2. Enable **Event Subscriptions**.
3. Select **Add New Event Subscription** and give it a name \(e.g. "Meeting Ended"\).
4. Select **Add events** and select the **End Meeting** event.

![event_types](https://github.com/mattermost/mattermost-plugin-zoom/assets/90389917/b2833ea8-fe0b-4c5e-9bba-62c281d928e4)

4. Enter a valid **Event notification endpoint URL** \(`https://SITEURL/plugins/zoom/webhook?secret=WEBHOOKSECRET`\).
   * `SITEURL` should be your Mattermost server URL.
   * `WEBHOOKSECRET` is generated when setting up Mattermost to use the plugin.

![mattermost_webhook_secret](https://github.com/mattermost/mattermost-plugin-zoom/assets/74422101/58b9ac74-ecf9-4e3e-986e-94fd4c39bfb0)

Select **Save** and then save your app.

### Mattermost Setup

**Note:** You need a paid Zoom account to use the plugin.

#### First steps

* Enable settings for [overriding usernames](https://docs.mattermost.com/configure/integrations-configuration-settings.html#integrate-enableusernameoverride) and [overriding profile picture icons](https://docs.mattermost.com/configure/integrations-configuration-settings.html#enable-integrations-to-override-profile-picture-icons).
* Go to **System Console > Plugins > Zoom** to configure the Zoom Plugin.

[Mattermost configuration settings](https://github.com/mattermost/mattermost-plugin-zoom/assets/74422101/f46aa92c-f263-4e0c-8c71-a3910281268a")

#### Plugin configuration

* Set **Enable Plugin** to `true`.
* How are you hosting Zoom?
  * **Self Hosted?**
    * If you're using a self-hosted private cloud or on-premises Zoom server, enter the **Zoom URL** and **Zoom API URL** for the Zoom server, for example `https://yourzoom.com` and `https://api.yourzoom.com/v2` respectively. Leave blank if you're using Zoom's vendor-hosted SaaS service.
  * **Cloud Hosted?**
    * Leave **Zoom API URL** and **Zoom URL** fields blank.
* If you are using an account level app on Zoom, set **OAuth by Account Level App** to `true`.
* Connect your users to Zoom using OAuth.
  * Use the Client ID and Client Secret generated when configuring Zoom to fill in the fields **Zoom OAuth Client ID** and **Zoom OAuth Client Secret**. (If you have selected the app type as "Account level app", make sure that you are using an account-level app on Zoom side as well)
  * Select the **Regenerate** button next to the field **At Rest Token Encryption Key**.
* If you are using Webhooks or Deauthorization, make sure you select the **Regenerate** button on **Webhook Secret** field. Then set the **Zoom Webhook Secret** from the features page in your Zoom app.
* Select **Save**.

## User guide

### Connect your Account

The first time you create a meeting, you may be required to connect your account. Follow the instructions to connect your Zoom account.

### Start Meetings

Once enabled, selecting the video icon in a Mattermost channel invites team members to join a Zoom call, hosted using the credentials of the user who initiated the call.

### Slash Command

You can also start a meeting in any chat window by typing `/zoom start`.

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
