import { ApplicationConfig, provideZoneChangeDetection } from '@angular/core';
import { provideRouter } from '@angular/router';
import { provideEffects } from '@ngrx/effects';
import { provideStore } from '@ngrx/store';
import { routes } from './app.routes';
import { provideAnimationsAsync } from '@angular/platform-browser/animations/async';
import { AppEffects } from './store/effects/app.effects';
import { appReducer } from './store/reducers/app.reducers';

export const appConfig: ApplicationConfig = {
  providers: [
    provideAnimationsAsync(),
    provideEffects([AppEffects]),
    provideStore({ app: appReducer }),
    provideRouter(routes),
    provideZoneChangeDetection({ eventCoalescing: true })
  ]
};
