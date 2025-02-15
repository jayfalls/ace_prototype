import { createActionGroup, props, emptyProps } from "@ngrx/store";
import { ILLMModelProvider } from "../../models/model-provider.models";

export const modelProviderActions = createActionGroup({
    source: "model_provider",
    events: {
        getLLMModels: emptyProps(),
        getLLMModelsSuccess: props<{ models: ILLMModelProvider[] }>(),
        getLLMModelsFailure: props<{ error: Error }>(),
        getLLMModelTypes: emptyProps(),
        getLLMModelTypesSuccess: props<{ model_types: string[] }>(),
        getLLMModelTypesFailure: props<{ error: Error }>()
    },
});
