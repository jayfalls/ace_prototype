import { createReducer, on } from "@ngrx/store";
import { onLoadableError, onLoadableLoad, onLoadableSuccess } from "../state/loadable.state";
import { appActions } from "../actions/app.actions";
import { createInitialAppState } from "../state/app.state";

export const appReducer = createReducer(
    createInitialAppState(),
    on(appActions.getACEVersionData, state => ({...onLoadableLoad(state)})),
    on(appActions.getACEVersionDataSuccess, (state, { versionData }) => ({...onLoadableSuccess(state), versionData: {...versionData}})),
    on(appActions.getACEVersionDataFailure, (state, { error }) => ({...onLoadableError(state, error)}))
)
