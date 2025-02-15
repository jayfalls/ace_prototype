import { inject, Injectable } from "@angular/core";
import { createEffect, ofType, Actions } from "@ngrx/effects";
import { map, catchError, of, switchMap } from "rxjs";
import { modelProviderActions } from "./model-provider.actions";
import { ILLMModelProvider } from "../../models/model-provider.models";
import { ModelProviderService } from "../../services/model-provider.service";


@Injectable()
export class modelProviderEffects {
    private actions$ = inject(Actions);

    getLLMModels$ = createEffect(() => this.actions$.pipe(
        ofType(modelProviderActions.getLLMModels),
        switchMap(() => this.modelProviderService.getLLMModels().pipe(
            map((models: ILLMModelProvider[]) => modelProviderActions.getLLMModelsSuccess({ models })),
            catchError(error => of(modelProviderActions.getLLMModelsFailure({ error })))
        ))
    ))

    getLLMModelTypes$ = createEffect(() => this.actions$.pipe(
        ofType(modelProviderActions.getLLMModelTypes),
        switchMap(() => this.modelProviderService.getLLMModelTypes().pipe(
            map((model_types: string[]) => modelProviderActions.getLLMModelTypesSuccess({ model_types })),
            catchError(error => of(modelProviderActions.getLLMModelTypesFailure({ error })))
        ))
    ))

    constructor(
        private modelProviderService: ModelProviderService
    ) { }
}
