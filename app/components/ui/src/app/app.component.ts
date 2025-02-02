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
import { Observable } from "rxjs";
//// Local
import { appActions } from "./store/actions/app.actions";
import { selectAppState } from './store/selectors/app.selectors';
import { AppState } from './store/state/app.state';


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
  versionData$: Observable<AppState>;
  version: string = "0";

  // Initialisation
  constructor(private store: Store) {
    this.versionData$ = this.store.select(selectAppState);
  }

  ngOnInit(): void {
      this.store.dispatch(appActions.getACEVersionData());
      this.versionData$.subscribe( versionData => this.version = versionData.versionData.version);
  }
}
