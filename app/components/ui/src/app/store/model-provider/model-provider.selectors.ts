import { createFeatureSelector, createSelector } from "@ngrx/store";
import { ModelProviderState } from "../../state/model-provider.state";

export const selectModelProviderState = createFeatureSelector<ModelProviderState>("model_provider");
export const selectLLMModels = createSelector(selectModelProviderState, (state: ModelProviderState) => state.llm.models);
export const selectLLMModelTypes = createSelector(selectModelProviderState, (state: ModelProviderState) => state.llm.model_types);
