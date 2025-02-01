import { createDefaultLoadable, Loadable } from "./loadable.state";
import { IACEVersionData } from "../../models/app.models";

export interface IAppState extends Loadable {
    versionData: IACEVersionData;
}

export function createInitialAppState(): IAppState {
    return {
        ...createDefaultLoadable(),
        versionData: {
            version: "0"
        }
    }
}


