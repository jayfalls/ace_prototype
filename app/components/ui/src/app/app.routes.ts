import { Routes } from "@angular/router";
import { HomeComponent } from "./pages/home/home.component";
import { DashboardComponent } from "./pages/dashboard/dashboard.component";
import { ChatComponent } from "./pages/chat/chat.component";
import { SettingsComponent } from "./pages/settings/settings.component";

export const routes: Routes = [
    { path: "", component: HomeComponent },
    { path: "dashboard", component: DashboardComponent },
    { path: "chat", component: ChatComponent },
    { path: "settings", component: SettingsComponent },
];
