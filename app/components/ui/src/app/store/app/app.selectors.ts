import { createSelector, createFeatureSelector } from "@ngrx/store";
import { AppState } from "../../state/app.state";

export const selectAppState = createFeatureSelector<AppState>("app_data");
export const selectAppVersionDataState = createSelector(selectAppState, (state: AppState) => state.version_data);
