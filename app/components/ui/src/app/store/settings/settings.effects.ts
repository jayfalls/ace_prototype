import { inject, Injectable } from "@angular/core";
import { createEffect, ofType, Actions } from "@ngrx/effects";
import { map, catchError, of, switchMap } from "rxjs";
import { settingsActions } from "./settings.actions";
import { ISettings } from "../../models/settings.models";
import { SettingsService } from "../../services/settings.service";


@Injectable()
export class SettingsEffects {
    private actions$ = inject(Actions);

    getSettings$ = createEffect(() => this.actions$.pipe(
        ofType(settingsActions.getSettings),
        switchMap(() => this.settingsService.getSettings().pipe(
            map((settings: ISettings) => settingsActions.getSettingsSuccess({ settings })),
            catchError(error => of(settingsActions.getSettingsFailure({ error })))
        ))
    ))

    constructor(
        private settingsService: SettingsService
    ) { }
}
