# Webhook Configuration

## Configure webhook events

When a Zoom meeting ends, the original link shared in the channel can be changed to indicate the meeting has ended and how long it lasted. To enable this functionality, you can create a webhook subscription in Zoom that tells the Mattermost server every time a meeting ends. The Mattermost server then updates the original Zoom message.

1. Select **Feature**.
2. Enable **Event Subscriptions**.
3. Select **Add New Event Subscription** and give it a name \(e.g. "Meeting Ended"\).
4. Enter a valid **Event notification endpoint URL** \(`https://SITEURL/plugins/zoom/webhook?secret=WEBHOOKSECRET`\).
   * `SITEURL` should be your Mattermost server URL.
   * `WEBHOOKSECRET` is generated during [Mattermost Setup](../mattermost-setup.md).

![Feature screen](../../.gitbook/assets/feature.png)

5. Select **Add events** and select the **End Meeting** event.

![Event types screen](../../.gitbook/assets/event_types.png)

6. Select **Done** and then save your app.
