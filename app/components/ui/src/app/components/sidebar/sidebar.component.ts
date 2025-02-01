import { Component, signal } from "@angular/core";
import { MatButtonModule } from "@angular/material/button";
import { MatIconModule } from "@angular/material/icon";
import { MatListModule } from "@angular/material/list";
import { MatTooltipModule } from "@angular/material/tooltip";
import { MatSidenavModule } from "@angular/material/sidenav";
import { RouterModule, RouterOutlet } from "@angular/router";


export type SidebarItem = {
  name: string,
  icon: string,
  route?: string,
}


@Component({
  selector: "ace-sidebar",
  templateUrl: "sidebar.component.html",
  styleUrl: "sidebar.component.scss",
  imports: [
      MatButtonModule,
      MatIconModule,
      MatListModule,
      MatTooltipModule,
      MatSidenavModule,
      RouterModule,
      RouterOutlet
  ],
})
export class ACESidebarComponent {
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
            name: "Settings",
            icon: "settings",
            route: "settings"
        }
    ])
}
