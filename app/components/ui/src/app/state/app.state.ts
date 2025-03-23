import { createDefaultLoadable, Loadable } from "./loadable.state";
import { IAppVersionData } from "../models/app.models";

export interface AppState extends Loadable {
    version_data: IAppVersionData;
}

export function createInitialAppState(): AppState {
    return {
        ...createDefaultLoadable(),
        version_data: {
            version: "0",
            authors: [],
            license: "",
            last_update: "",
            rebuild_date: ""
        }
    }
}


