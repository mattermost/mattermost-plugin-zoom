---
description: >-
  You'll need to choose which authnetication method you want your users to
  connect their Zoom accounts
---

# Zoom Configuration

Zoom supports two authentication methods for your users to connect Mattermost and Zoom. Choose between using OAuth and JWT/Password based authentication:

1. [Oauth: ](zoom-setup-oauth.md)
   * Each user will need to connect their Zoom account with their Mattermost account before they can use the integration.  When they try to create a meeting for the first time, they will receive a message to connect their account, and will need to click "Approve" on a pop-up web page.
   * Users can connect their MM/Zoom accounts even if their emails do not match
2. [JWT/Password](zoom-setup-jwt.md)
   * Users don't have to connect their account to use the integration which makes it easy to get started.  
   * Uses JWT to pass security tokens.  This may not be sufficiently secure for some customers.



