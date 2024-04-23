export function handleMeetingStarted(msg: {data: {meeting_url: string}}) {
    window.open(msg.data.meeting_url);
}
