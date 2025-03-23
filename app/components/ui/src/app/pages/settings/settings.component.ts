// DEPENDENCIES
//// Angular
import { Component, OnInit } from "@angular/core";
import { Store } from "@ngrx/store";
import { filter, take } from "rxjs";
import { FormArray, FormBuilder, FormGroup, ReactiveFormsModule, Validators } from "@angular/forms";
import { MatButtonModule } from "@angular/material/button";
import { MatDividerModule } from "@angular/material/divider";
import { MatIconModule } from "@angular/material/icon";
import { MatInputModule } from "@angular/material/input";
import { MatFormFieldModule } from "@angular/material/form-field";
import { MatSelectModule } from "@angular/material/select";
import { MatSlideToggleModule } from "@angular/material/slide-toggle";
//// Local
import { IAppVersionData } from "../../models/app.models";
import { ILLMModelProvider } from "../../models/model-provider.models";
import { IModelProviderSetting, ISettings } from "../../models/settings.models";
import { modelProviderActions } from "../../store/model-provider/model-provider.actions";
import { selectAppVersionDataState } from "../../store/app/app.selectors";
import { selectLLMModelTypes } from "../../store/model-provider/model-provider.selectors";
import { settingsActions } from "../../store/settings/settings.actions";
import { selectSettingsState } from "../../store/settings/settings.selectors";

@Component({
  selector: "page-settings",
  imports: [
    ReactiveFormsModule,
    MatButtonModule,
    MatDividerModule,
    MatIconModule,
    MatInputModule,
    MatFormFieldModule,
    MatSelectModule,
    MatSlideToggleModule
  ],
  templateUrl: "./settings.component.html",
  styleUrl: "./settings.component.scss"
})
export class SettingsComponent implements OnInit {
  appVersionData!: IAppVersionData;
  llmModels: ILLMModelProvider[] = [];
  llmModelTypes: string[] = [];
  selectedLLMModelType: string = "";
  settings!: ISettings;

  settingsForm!: FormGroup;
  generalForm!: FormGroup;
  uiSettingsForm!: FormGroup;
  modelProviderForm!: FormGroup;
  layerSettingsForm!: FormArray;

  changesDetected: boolean = false;

  constructor(
    private formBuilder: FormBuilder,
    private store: Store
  ) {
    this.initialiseForm();
  }

  ngOnInit(): void {
    this.store.select(selectAppVersionDataState).subscribe( version_data => this.appVersionData = version_data );
    this.store.dispatch(modelProviderActions.getLLMModels());
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
    this.generalForm = this.formBuilder.group({
      ace_name: ["", [Validators.required, Validators.maxLength(32)]]
    });

    this.uiSettingsForm = this.formBuilder.group({
      dark_mode: [true],
      show_footer: [true]
    });

    this.layerSettingsForm = this.formBuilder.array([]);

    this.modelProviderForm = this.formBuilder.group({
      individual_provider_settings: this.formBuilder.array([]),
      three_d_model_type_settings: this.formBuilder.array([]),
      audio_model_type_settings: this.formBuilder.array([]),
      image_model_type_settings: this.formBuilder.array([]),
      llm_model_type_settings: this.formBuilder.array([]),
      rag_model_type_settings: this.formBuilder.array([])
    });

    this.settingsForm = this.formBuilder.group({
      ...this.generalForm.controls,
      ui_settings: this.uiSettingsForm,
      layer_settings: this.layerSettingsForm,
      model_provider_settings: this.modelProviderForm
    });

    this.settingsForm.valueChanges.subscribe(() => {
      this.changesDetected = true;
    });
  }

  private patchFormValues(): void {
    if (!this.settings) return;

    this.initialiseForm();

    this.generalForm.patchValue({
      ace_name: this.settings.ace_name,
    });

    this.uiSettingsForm.patchValue({
      dark_mode: this.settings.ui_settings.dark_mode,
      show_footer: this.settings.ui_settings.show_footer
    });

    const modelProviderSettings: IModelProviderSetting = this.settings.model_provider_settings;
    this.individualModelProviderSettingsControl?.clear();

    modelProviderSettings.individual_provider_settings.forEach(provider => {
      this.individualModelProviderSettingsControl?.push(
        this.formBuilder.group({
          name: [provider.name],
          enabled: [provider.enabled],
          api_key: [provider.api_key]
        })
      );
    });

    this.layerSettingsForm.clear();
    this.settings.layer_settings.forEach(layer => {
      this.layerSettingsForm.push(
        this.formBuilder.group({
          layer_name: [layer?.layer_name || "", Validators.required],
          model_type: [layer?.model_type || "", Validators.required]
        })
      );
    });

    this.settingsForm.markAsPristine();
    this.changesDetected = false;
  }

  formatSnakeCase(layerName?: string): string {
    return (layerName || "")
      .split("_")
      .map(word => word.charAt(0).toUpperCase() + word.slice(1).toLowerCase())
      .join(" ");
  }

  onReset() {
    this.store.dispatch(settingsActions.resetSettings());
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

    this.store.dispatch(settingsActions.editSettings({ settings: updatedSettings }));
  }

  get aceNameControl() {
    return this.generalForm.get("ace_name");
  }

  get individualModelProviderSettingsControl(): FormArray {
    return this.modelProviderForm.get("individual_provider_settings") as FormArray;
  }

  get layerSettingsControl(): FormArray {
    return this.layerSettingsForm;
  }
}
