// DEPENDENCIES
//// Angular
import { TitleCasePipe } from "@angular/common";
import { Component, OnInit } from "@angular/core";
import { Store } from "@ngrx/store";
import { Observable } from "rxjs";
import {FormsModule} from '@angular/forms';
import { MatCardModule } from "@angular/material/card";
import {MatFormFieldModule} from '@angular/material/form-field';
import {MatInputModule} from '@angular/material/input';
import { MatListModule } from "@angular/material/list";
import {MatSelectModule} from '@angular/material/select';
//// Local
import { ISettings } from "../../models/settings.models";
import { settingsActions } from "../../store/settings/settings.actions";
import { selectSettingsState } from "../../store/settings/settings.selectors";
import { SettingsState } from "../../state/settings.state";

@Component({
  selector: "page-settings",
  imports: [FormsModule, MatCardModule, MatFormFieldModule, MatInputModule, MatListModule, MatSelectModule, TitleCasePipe],
  templateUrl: "./settings.component.html",
  styleUrl: "./settings.component.scss"
})
export class SettingsComponent implements OnInit {
  settings$: Observable<SettingsState>;

  settings!: ISettings;

  constructor(private store: Store) {
    this.settings$ = this.store.select(selectSettingsState);
  }

  ngOnInit(): void {
    this.store.dispatch(settingsActions.getSettings());
    this.settings$.subscribe( settings => this.settings = settings.settings);
  }

  formatLayerNames(layerName: string): string {
    return layerName
    .split('_')
    .map(word => word.charAt(0).toUpperCase() + word.slice(1).toLowerCase())
    .join(' ');
  }
}
