// DEPENDENCIES
//// Angular
import { TitleCasePipe } from "@angular/common";
import { Component, OnInit } from "@angular/core";
import { Store } from "@ngrx/store";
import { filter, take } from "rxjs";
import { FormArray, FormBuilder, FormGroup, ReactiveFormsModule, Validators } from "@angular/forms";
import { MatButtonModule } from "@angular/material/button";
import { MatCheckboxModule } from "@angular/material/checkbox";
import { MatDividerModule } from "@angular/material/divider";
import { MatIconModule } from "@angular/material/icon";
import { MatFormFieldModule } from "@angular/material/form-field";
import { MatInputModule } from "@angular/material/input";
import { MatSelectModule } from "@angular/material/select";
//// Local
import { ILLMModelProvider } from "../../models/model-provider.models";
import { ISettings } from "../../models/settings.models";
import { modelProviderActions } from "../../store/model-provider/model-provider.actions";
import { selectLLMModelTypes } from "../../store/model-provider/model-provider.selectors";
import { settingsActions } from "../../store/settings/settings.actions";
import { selectSettingsState } from "../../store/settings/settings.selectors";

@Component({
  selector: "page-settings",
  imports: [
    ReactiveFormsModule,
    MatButtonModule,
    MatCheckboxModule,
    MatDividerModule,
    MatIconModule,
    MatFormFieldModule,
    MatInputModule,
    MatSelectModule,
    TitleCasePipe
  ],
  templateUrl: "./settings.component.html",
  styleUrl: "./settings.component.scss"
})
export class SettingsComponent implements OnInit {
  llmModels: ILLMModelProvider[] = [];
  llmModelTypes: string[] = [];
  selectedLLMModelType: string = "";
  settings!: ISettings;
  settingsForm!: FormGroup;

  changesDetected: boolean = false;

  constructor(
    private formBuilder: FormBuilder,
    private store: Store
  ) {
    this.initialiseForm();
  }

  ngOnInit(): void {
    this.store.dispatch(modelProviderActions.getLLMModelTypes());
    this.store.select(selectLLMModelTypes).pipe(
      filter((model_types: string[]): model_types is string[] => model_types.length > 0),
      take(1)
    ).subscribe(model_types => {
      this.llmModelTypes = model_types;
      this.initialiseForm();
    });

    this.store.dispatch(settingsActions.getSettings());
    this.store.select(selectSettingsState).subscribe(settings => {
      this.settings = settings.settings;
      this.patchFormValues();
    });
  }

  private initialiseForm(): void {
    this.settingsForm = this.formBuilder.group({
      ace_name: ["", [Validators.required, Validators.maxLength(32)]],
      ui_settings: this.formBuilder.group({
        show_footer: [true]
      }),
      layer_settings: this.formBuilder.array([])
    });

    this.settingsForm.valueChanges.subscribe(() => {
      this.changesDetected = true;
    })
  }

  private createLayerSettingGroup(layer: any): FormGroup {
    return this.formBuilder.group({
      layer_name: [layer?.layer_name || "", Validators.required],
      model_type: [layer?.model_type || "", Validators.required]
    });
  }

  private patchFormValues(): void {
    if (!this.settings) return;

    this.settingsForm.patchValue({
      ace_name: this.settings.ace_name,
      ui_settings: {
        show_footer: this.settings.ui_settings.show_footer
      }
    });

    const layerArray = this.layerSettingsControl;
    layerArray.clear();

    this.settings.layer_settings.forEach(layer => {
      layerArray.push(this.createLayerSettingGroup(layer));
    });

    this.settingsForm.markAsPristine();
    this.changesDetected = false;
  }

  formatLayerNames(layerName?: string): string {
    return (layerName || "")
      .split("_")
      .map(word => word.charAt(0).toUpperCase() + word.slice(1).toLowerCase())
      .join(" ");
  }

  onSave() {
    if (!this.settingsForm.valid) {
      return;
    }
    const updatedSettings = {
      ...this.settings,
      ...this.settingsForm.value
    };

    this.changesDetected = false;
    this.settingsForm.markAsPristine();

    // this.store.dispatch(settingsActions.editSettings({ settings: updatedSettings }));
  }

  get aceNameControl() {
    return this.settingsForm.get("ace_name");
  }

  get uiSettingsControl() {
    return this.settingsForm.get("ui_settings");
  }

  get layerSettingsControl(): FormArray {
    return this.settingsForm.get("layer_settings") as FormArray;
  }
}
