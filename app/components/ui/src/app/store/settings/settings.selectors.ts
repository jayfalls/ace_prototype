import { createFeatureSelector } from "@ngrx/store";
import { SettingsState } from "../../state/settings.state";

export const selectSettingsState = createFeatureSelector<SettingsState>("settings");
