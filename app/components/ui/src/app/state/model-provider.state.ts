import { createDefaultLoadable, Loadable } from "./loadable.state";
import { ILLMModelProvider } from "../models/model-provider.models";

export interface ModelProviderLLMState extends Loadable {
  models: ILLMModelProvider[],
  model_types: string[]
}

export interface ModelProviderState extends Loadable {
  llm: ModelProviderLLMState
}

export function createInitialModelProviderState(): ModelProviderState {
  return {
    ...createDefaultLoadable(),
    llm: {
      ...createDefaultLoadable(),
      models: [],
      model_types: []
    }
  }
}
