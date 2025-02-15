import { createReducer, on } from "@ngrx/store";
import { onLoadableError, onLoadableLoad, onLoadableSuccess } from "../../state/loadable.state";
import { settingsActions } from "./settings.actions";
import { createInitialSettingsState } from "../../state/settings.state";

export const settingsReducer = createReducer(
    createInitialSettingsState(),
    on(settingsActions.getSettings, state => ({...onLoadableLoad(state)})),
    on(settingsActions.getSettingsSuccess, (state, { settings }) => ({...onLoadableSuccess(state), settings: {...settings}})),
    on(settingsActions.getSettingsFailure, (state, { error }) => ({...onLoadableError(state, error)}))
)
