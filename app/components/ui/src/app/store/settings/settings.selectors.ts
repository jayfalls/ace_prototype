import { createFeatureSelector, createSelector } from "@ngrx/store";
import { SettingsState } from "../../state/settings.state";

export const selectSettingsState = createFeatureSelector<SettingsState>("settings");
export const selectUISettingsState = createSelector(selectSettingsState, (state: SettingsState) => state.settings.ui_settings);
