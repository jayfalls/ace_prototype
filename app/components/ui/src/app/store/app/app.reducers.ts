import { createReducer, on } from "@ngrx/store";
import { onLoadableError, onLoadableLoad, onLoadableSuccess } from "../../state/loadable.state";
import { appActions } from "./app.actions";
import { createInitialAppState } from "../../state/app.state";

export const appReducer = createReducer(
    createInitialAppState(),
    on(appActions.getAppVersionData, state => ({...onLoadableLoad(state)})),
    on(appActions.getAppVersionDataSuccess, (state, { version_data }) => ({...onLoadableSuccess(state), version_data: {...version_data}})),
    on(appActions.getAppVersionDataFailure, (state, { error }) => ({...onLoadableError(state, error)}))
)
