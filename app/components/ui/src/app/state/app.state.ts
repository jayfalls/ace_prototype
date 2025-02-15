import { createDefaultLoadable, Loadable } from "./loadable.state";
import { IACEVersionData } from "../models/app.models";

export interface AppState extends Loadable {
    versionData: IACEVersionData;
}

export function createInitialAppState(): AppState {
    return {
        ...createDefaultLoadable(),
        versionData: {
            version: "0"
        }
    }
}


