import { createSelector, createFeatureSelector } from "@ngrx/store";
import { IAppState } from "../state/app.state";

export const selectAppState = createFeatureSelector<IAppState>("app");
export const selectACEVersionData = createSelector(selectAppState, (state: IAppState) => state.versionData);
