// DEPENDENCIES
//// Angular
import { ApplicationConfig, provideZoneChangeDetection } from '@angular/core';
import { provideHttpClient } from '@angular/common/http';
import { provideRouter } from '@angular/router';
import { provideEffects } from '@ngrx/effects';
import { provideStore } from '@ngrx/store';
import { provideStoreDevtools } from '@ngrx/store-devtools';
//// Local
import { routes } from './app.routes';
import { provideAnimationsAsync } from '@angular/platform-browser/animations/async';
import { AppEffects } from './store/app/app.effects';
import { appReducer } from './store/app/app.reducers';


// CONFIG
export const appConfig: ApplicationConfig = {
  providers: [
    provideAnimationsAsync(),
    provideEffects([AppEffects]),
    provideHttpClient(),
    provideStore({ app_data: appReducer }),
    provideStoreDevtools({
      maxAge: 25,
      logOnly: false
    }),
    provideRouter(routes),
    provideZoneChangeDetection({ eventCoalescing: true })
  ]
};
