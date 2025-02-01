import { Component, OnInit } from "@angular/core";
import { Store } from "@ngrx/store";
import { appActions } from "./store/actions/app.actions";
import { ACESidebarComponent } from "./components/sidebar/sidebar.component";

@Component({
  selector: "app-root",
  imports: [
    ACESidebarComponent
  ],
  templateUrl: "./app.component.html",
  styleUrl: "./app.component.scss"
})
export class AppComponent implements OnInit {
  title = "ACE";

  constructor(private store: Store) {}

  ngOnInit(): void {
      this.store.dispatch(appActions.getACEVersionData());
  }
}
