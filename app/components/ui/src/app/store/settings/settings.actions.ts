import { createActionGroup, props, emptyProps } from "@ngrx/store";
import { ISettings } from "../../models/settings.models";

export const settingsActions = createActionGroup({
    source: "settings",
    events: {
        getSettings: emptyProps(),
        getSettingsSuccess: props<{ settings: ISettings }>(),
        getSettingsFailure: props<{ error: Error }>()
    },
});
