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
import { ILLMModelProvider } from "../../models/model-provider.models";
import { ISettings } from "../../models/settings.models";
import { modelProviderActions } from "../../store/model-provider/model-provider.actions";
import { selectLLMModels, selectLLMModelTypes } from "../../store/model-provider/model-provider.selectors";
import { settingsActions } from "../../store/settings/settings.actions";
import { selectSettingsState } from "../../store/settings/settings.selectors";

@Component({
  selector: "page-settings",
  imports: [FormsModule, MatCardModule, MatFormFieldModule, MatInputModule, MatListModule, MatSelectModule, TitleCasePipe],
  templateUrl: "./settings.component.html",
  styleUrl: "./settings.component.scss"
})
export class SettingsComponent implements OnInit {
  llmModels: ILLMModelProvider[] = [];
  llmModelTypes: string[] = [];
  settings!: ISettings;

  constructor(private store: Store) {}

  ngOnInit(): void {
    this.store.dispatch(settingsActions.getSettings());
    this.store.select(selectSettingsState).subscribe( settings => this.settings = settings.settings);
    this.store.dispatch(modelProviderActions.getLLMModels());
    this.store.select(selectLLMModels).subscribe(llmModels => this.llmModels = llmModels);
    this.store.dispatch(modelProviderActions.getLLMModelTypes());
    this.store.select(selectLLMModelTypes).subscribe(llmModelTypes => this.llmModelTypes = llmModelTypes);
  }

  formatLayerNames(layerName: string): string {
    return layerName
    .split('_')
    .map(word => word.charAt(0).toUpperCase() + word.slice(1).toLowerCase())
    .join(' ');
  }
}
