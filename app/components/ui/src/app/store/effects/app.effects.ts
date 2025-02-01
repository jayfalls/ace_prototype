import { inject, Injectable } from "@angular/core";
import { createEffect, ofType, Actions } from "@ngrx/effects";
import { Store } from "@ngrx/store";
import { map, catchError, of, switchMap } from "rxjs";
import { appActions } from "../actions/app.actions";
import { IACEVersionData } from "../../models/app.models";
import { AppService } from "../../services/app.service";


@Injectable()
export class AppEffects {
    private actions$ = inject(Actions);
    private store$ = inject(Store);

    getACEVersionData$ = createEffect(() => this.actions$.pipe(
        ofType(appActions.getACEVersionData),
        switchMap(() => this.appService.getACEVersionData().pipe(
            map((versionData: IACEVersionData) => appActions.getACEVersionDataSuccess({ versionData })),
            catchError(error => of(appActions.getACEVersionDataFailure({ error })))
        ))
    ))

    constructor(
        private appService: AppService
    ) { }
}
