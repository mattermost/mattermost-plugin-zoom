// Copyright (c) 2019-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

export function handleMeetingStarted(msg: { data: { meeting_url: string } }) {
  window.open(msg.data.meeting_url, "_blank");
}
