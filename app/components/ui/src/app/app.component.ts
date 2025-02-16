// DEPENDENCIES
//// Angular
import { Component, inject, OnInit, signal } from "@angular/core";
import { MatButtonModule } from "@angular/material/button";
import { MatIconModule } from "@angular/material/icon";
import { MatListModule } from "@angular/material/list";
import { MatTooltipModule } from "@angular/material/tooltip";
import { MatSidenavModule } from "@angular/material/sidenav";
import { RouterModule, RouterOutlet } from "@angular/router";
import { Store } from "@ngrx/store";
//// Local
import { ThemeService } from "./theme";
import { IAppVersionData } from "./models/app.models";
import { ISettings, IUISettings } from "./models/settings.models";
import { appActions } from "./store/app/app.actions";
import { settingsActions } from "./store/settings/settings.actions";
import { selectAppVersionDataState } from './store/app/app.selectors';
import { selectSettingsState } from "./store/settings/settings.selectors";


// TYPES
export type SidebarItem = {
  name: string,
  icon: string,
  route?: string,
}


// COMPONENT
@Component({
  selector: "app-root",
  imports: [
    MatButtonModule,
    MatIconModule,
    MatListModule,
    MatSidenavModule,
    MatTooltipModule,
    RouterModule,
    RouterOutlet
  ],
  templateUrl: "./app.component.html",
  styleUrl: "./app.component.scss"
})
export class AppComponent implements OnInit {
  // Variables
  title = "ACE";
  themeService = inject(ThemeService);
  sidebarItems = signal<SidebarItem[]>([
    {
      name: "Home",
      icon: "home",
      route: ""
    },
    {
      name: "Dashboard",
      icon: "dashboard",
      route: "dashboard"
    },
    {
      name: "Chat",
      icon: "chat",
      route: "chat"
    },
    {
      name: "Model Garden",
      icon: "local_florist",
      route: "model-garden"
    },
    {
      name: "Settings",
      icon: "settings",
      route: "settings"
    }
  ])

  settings?: ISettings;
  uiSettings?: IUISettings;
  appVersionData?: IAppVersionData;

  // Initialisation
  constructor(private store: Store) {}

  ngOnInit(): void {
      this.store.dispatch(appActions.getAppVersionData());
      this.store.select(selectAppVersionDataState).subscribe( version_data => this.appVersionData = version_data );
      this.store.dispatch(settingsActions.getSettings());
      this.store.select(selectSettingsState).subscribe( settings => {
        this.settings = settings.settings;
        this.uiSettings = settings.settings.ui_settings;
        this.themeService.setDarkMode(this.uiSettings.dark_mode);
      })
  }

  toggleDarkMode() {
    if (!this.settings) {
      return;
    }
    this.store.dispatch(settingsActions.editSettings({
      settings: {
        ...this.settings,
        ui_settings: {
          ...this.settings.ui_settings,
          dark_mode: !this.settings.ui_settings.dark_mode
        }
      }
    }))
  }
}
