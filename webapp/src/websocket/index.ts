import {openScheduleMeetingModal} from "@/actions";

export function handleOpenScheduleMeetingDialog(store: any) {
    return (msg: any) => {
        if (!msg.data) {
            return;
        }

        openScheduleMeetingModal(msg.ChannelId)(store.dispatch);
    };
}
