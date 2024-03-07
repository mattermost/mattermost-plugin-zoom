import {openScheduleMeetingModal} from "@/actions";
import {Store} from "redux";

export function handleOpenScheduleMeetingDialog(store: Store) {
    return () => {
        openScheduleMeetingModal()(store.dispatch);
    };
}
