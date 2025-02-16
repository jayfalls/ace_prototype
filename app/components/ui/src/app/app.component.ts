// DEPENDENCIES
//// Angular
import { Component, OnInit, signal } from "@angular/core";
import { MatButtonModule } from "@angular/material/button";
import { MatIconModule } from "@angular/material/icon";
import { MatListModule } from "@angular/material/list";
import { MatTooltipModule } from "@angular/material/tooltip";
import { MatSidenavModule } from "@angular/material/sidenav";
import { RouterModule, RouterOutlet } from "@angular/router";
import { Store } from "@ngrx/store";
//// Local
import { IAppVersionData } from "./models/app.models";
import { IUISettings } from "./models/settings.models";
import { appActions } from "./store/app/app.actions";
import { settingsActions } from "./store/settings/settings.actions";
import { selectAppVersionDataState } from './store/app/app.selectors';
import { selectUISettingsState } from "./store/settings/settings.selectors";


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
    MatTooltipModule,
    MatSidenavModule,
    RouterModule,
    RouterOutlet
  ],
  templateUrl: "./app.component.html",
  styleUrl: "./app.component.scss"
})
export class AppComponent implements OnInit {
  // Variables
  title = "ACE";
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

  uiSettings?: IUISettings;
  appVersionData?: IAppVersionData;

  // Initialisation
  constructor(private store: Store) {}

  ngOnInit(): void {
      this.store.dispatch(appActions.getAppVersionData());
      this.store.select(selectAppVersionDataState).subscribe( version_data => this.appVersionData = version_data );
      this.store.dispatch(settingsActions.getSettings());
      this.store.select(selectUISettingsState).subscribe( ui_settings => this.uiSettings = ui_settings );
  }
}
