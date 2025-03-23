import { createActionGroup, props, emptyProps } from "@ngrx/store";
import { IAppVersionData } from "../../models/app.models";

export const appActions = createActionGroup({
    source: "app",
    events: {
        getAppVersionData: emptyProps(),
        getAppVersionDataSuccess: props<{ version_data: IAppVersionData }>(),
        getAppVersionDataFailure: props<{ error: Error }>()
    },
});
