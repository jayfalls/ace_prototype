// DEPENDENCIES
//// Angular
import { inject, Injectable } from "@angular/core";
import { createEffect, ofType, Actions } from "@ngrx/effects";
import { map, catchError, of, switchMap } from "rxjs";
//// Local
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

    editSettings$ = createEffect(() => this.actions$.pipe(
        ofType(settingsActions.editSettings),
        switchMap(({ settings }) => this.settingsService.editSettings(settings).pipe(
            map(() => settingsActions.getSettings()),
            catchError(error => of(settingsActions.editSettingsFailure({ error })))
        ))
    ))

    editSettingsSuccess$ = createEffect(() => this.actions$.pipe(
      ofType(settingsActions.editSettingsSuccess),
      map(() => settingsActions.getSettings())
    ));

    resetSettings$ = createEffect(() => this.actions$.pipe(
        ofType(settingsActions.resetSettings),
        switchMap(() => this.settingsService.resetSettings().pipe(
            map(() => settingsActions.resetSettingsSuccess()),
            catchError(error => of(settingsActions.resetSettingsFailure({ error })))
        ))
    ))

    resetSettingsSuccess$ = createEffect(() => this.actions$.pipe(
      ofType(settingsActions.resetSettingsSuccess),
      map(() => settingsActions.getSettings())
    ));

    constructor(
        private settingsService: SettingsService
    ) { }
}
