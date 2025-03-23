import { inject, Injectable } from "@angular/core";
import { createEffect, ofType, Actions } from "@ngrx/effects";
import { map, catchError, of, switchMap } from "rxjs";
import { appActions } from "./app.actions";
import { IAppVersionData } from "../../models/app.models";
import { AppService } from "../../services/app.service";


@Injectable()
export class AppEffects {
    private actions$ = inject(Actions);

    getACEVersionData$ = createEffect(() => this.actions$.pipe(
        ofType(appActions.getAppVersionData),
        switchMap(() => this.appService.getAppVersionData().pipe(
            map((version_data: IAppVersionData) => appActions.getAppVersionDataSuccess({ version_data })),
            catchError(error => of(appActions.getAppVersionDataFailure({ error })))
        ))
    ))

    constructor(
        private appService: AppService
    ) { }
}
