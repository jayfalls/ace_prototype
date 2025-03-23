import { createReducer, on } from "@ngrx/store";
import { onLoadableError, onLoadableLoad, onLoadableSuccess } from "../../state/loadable.state";
import { modelProviderActions } from "./model-provider.actions";
import { createInitialModelProviderState } from "../../state/model-provider.state";

export const modelProviderReducer = createReducer(
    createInitialModelProviderState(),
    on(modelProviderActions.getLLMModels, state => ({
        ...state,
        llm: {
            ...state.llm,
            ...onLoadableLoad(state.llm)
        }
    })),
    on(modelProviderActions.getLLMModelsSuccess, (state, { models }) => ({
        ...state,
        llm: {
            ...state.llm,
            ...onLoadableSuccess(state.llm),
            models: models
        }
    })),
    on(modelProviderActions.getLLMModelsFailure, (state, { error }) => ({
        ...state,
        llm: {
            ...state.llm,
            ...onLoadableError(state.llm, error)
        }
    })),
    on(modelProviderActions.getLLMModelTypes, state => ({
        ...state,
        llm: {
            ...state.llm,
            ...onLoadableLoad(state.llm)
        }
    })),
    on(modelProviderActions.getLLMModelTypesSuccess, (state, { model_types }) => ({
        ...state,
        llm: {
            ...state.llm,
            ...onLoadableSuccess(state.llm),
            model_types: model_types
        }
    })),
    on(modelProviderActions.getLLMModelTypesFailure, (state, { error }) => ({
        ...state,
        llm: {
            ...state.llm,
            ...onLoadableError(state.llm, error)
        }
    }))
)
