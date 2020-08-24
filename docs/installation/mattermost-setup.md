---
description: Configuration steps for the Mattermost server
---

# Mattermost Setup

## Zoom Plugin Setup Guide

{% hint style="info" %}
You will need a paid Zoom account to use the plugin.
{% endhint %}

### First steps

* Enable settings for [overriding usernames](https://docs.mattermost.com/administration/config-settings.html#enable-integrations-to-override-usernames) and [overriding profile picture icons](https://docs.mattermost.com/administration/config-settings.html#enable-integrations-to-override-profile-picture-icons).
* Go to **System Console &gt; Plugins &gt; Zoom** to configure the Zoom Plugin.

![](../.gitbook/assets/image%20%281%29.png)

### Plugin configuration

* Set **Enable Plugin** to `true`
* How are you hosting Zoom?
  * **Self Hosted?**
    * If you're using a self-hosted private cloud or on-premise Zoom server, enter the **Zoom URL** and **Zoom API URL** for the Zoom server, for example `https://yourzoom.com` and `https://api.yourzoom.com/v2` respectively. Leave blank if you're using Zoom's vendor-hosted SaaS service.
  * **Cloud Hosted?**
    * Simply leave **Zoom API URL** and **Zoom URL** fields blank
* How are your users connecting to Zoom? \([more information](zoom-configuration/)\)
  * **OAuth?**
    * Set **Enable OAuth** to `true`
    * Use the Client ID and Client Secret generated during [Zoom Configuration](zoom-configuration/zoom-setup-oauth.md) to fill in the fields **Zoom OAuth Client ID** and **Zoom OAuth Client Secret**
    * Click the **Regenerate** button next to the field **At Rest Token Encryption Key**
    * Make sure **Enable Password based authentication** is set to `false`
    * Ignore **API Key** and **API Secret** fields
  * **JWT/Password?**
    * Make sure **Enable OAuth** is set to `false`
    * Ignore the fields **Zoom OAuth Client ID**, **Zoom OAuth Client Secret** and **At Rest Token Encryption Key**
    * Set **Enable Password based authentication** to `true`
    * Use the API Key and API Secret generated during [Zoom Configuration](zoom-configuration/zoom-setup-jwt.md) to fill in the fields **API Key** and **API Secret**.
* If you are using Webhooks or Deauthorization, make sure you hit the **Regenerate** button on **Webhook Secret** field.
* Click **Save**

