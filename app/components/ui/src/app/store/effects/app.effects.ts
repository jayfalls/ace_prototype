import { inject, Injectable } from "@angular/core";
import { createEffect, ofType, Actions } from "@ngrx/effects";
import { select, Store } from "@ngrx/store";
import { map, catchError, of, switchMap, filter, withLatestFrom, Observable, throwError } from "rxjs";
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
            catchError(() => of(appActions.getACEVersionDataFailure()))
        ))
    ))

    constructor(
        private appService: AppService
    ) { }
}
