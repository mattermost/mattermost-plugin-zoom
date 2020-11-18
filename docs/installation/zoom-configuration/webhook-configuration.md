# Webhook Configuration

## Configure Webhook Events

When a Zoom meeting ends, the original link shared in the channel can be changed to indicate the meeting has ended and how long it lasted. To enable this functionality, we need to create a webhook subscription in Zoom that tells the Mattermost server every time a meeting ends. The Mattermost server then updates the original Zoom message.

1. Click on **Feature**.
2. Enable **Event Subscriptions**.
3. Click **Add New Event Subscription** and give it a name \(e.g. "Meeting Ended"\).
4. Enter a valid **Event notification endpoint URL** \(`https://SITEURL/plugins/zoom/webhook?secret=WEBHOOKSECRET`\).
   * `SITEURL` should be your Mattermost server URL.
   * `WEBHOOKSECRET` is generated during [Mattermost Setup](../mattermost-setup.md).

![Feature screen](../../.gitbook/assets/screenshot-from-2020-06-05-19-51-56%20%282%29.png)

* Click **Add events** and select the **End Meeting** event.

![Event types screen](../../.gitbook/assets/screenshot-from-2020-06-05-20-43-04%20%282%29.png)

* Click **Done** and then save your app.

