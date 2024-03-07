export function handleMeetingStarted() {
    return (msg) => {
        if (!msg.data) {
            return;
        }

        window.open(msg.data.meeting_url)
    };
}
