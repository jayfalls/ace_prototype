import { createActionGroup, props, emptyProps } from "@ngrx/store";
import { IACEVersionData } from "../../models/app.models";

export const appActions = createActionGroup({
    source: "app",
    events: {
        getACEVersionData: emptyProps(),
        getACEVersionDataSuccess: props<{ versionData: IACEVersionData }>(),
        getACEVersionDataFailure: emptyProps()
    },
});
