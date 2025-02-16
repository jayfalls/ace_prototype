// DEPENDENCIES
//// Angular
import { TitleCasePipe } from "@angular/common";
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
    MatSlideToggleModule,
    TitleCasePipe
  ],
  templateUrl: "./settings.component.html",
  styleUrl: "./settings.component.scss"
})
export class SettingsComponent implements OnInit {
  private defaultIndividualModelProviderSettings = {
    enabled: false,
    api_key: ""
  }

  appVersionData!: IAppVersionData;
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
    this.store.select(selectAppVersionDataState).subscribe( version_data => this.appVersionData = version_data );
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
    if (this.settingsForm) {
      this.settingsForm.reset();
    }
    this.settingsForm = this.formBuilder.group({
      ace_name: ["", [Validators.required, Validators.maxLength(32)]],
      ui_settings: this.formBuilder.group({
        dark_mode: [true],
        show_footer: [true]
      }),
      layer_settings: this.formBuilder.array([]),
      model_provider_settings: this.formBuilder.group({
        claude_settings: this.formBuilder.group(this.defaultIndividualModelProviderSettings),
        deepseek_settings: this.formBuilder.group(this.defaultIndividualModelProviderSettings),
        google_vertex_ai_settings: this.formBuilder.group(this.defaultIndividualModelProviderSettings),
        grok_settings: this.formBuilder.group(this.defaultIndividualModelProviderSettings),
        groq_settings: this.formBuilder.group(this.defaultIndividualModelProviderSettings),
        ollama_settings: this.formBuilder.group(this.defaultIndividualModelProviderSettings),
        openai_settings: this.formBuilder.group(this.defaultIndividualModelProviderSettings),
        three_d_model_type_settings: this.formBuilder.array([]),
        audio_model_type_settings: this.formBuilder.array([]),
        image_model_type_settings: this.formBuilder.array([]),
        llm_model_type_settings: this.formBuilder.array([]),
        rag_model_type_settings: this.formBuilder.array([])
      })
    });

    this.settingsForm.valueChanges.subscribe(() => {
      this.changesDetected = true;
    })
  }

  private patchFormValues(): void {
    if (!this.settings) return;
    this.initialiseForm();

    // General
    this.settingsForm.patchValue({
      ace_name: this.settings.ace_name,
    });

    // UI
    this.settingsForm.patchValue({
      ui_settings: {
        dark_mode: this.settings.ui_settings.dark_mode,
        show_footer: this.settings.ui_settings.show_footer
      }
    });

    // Layers
    let layerArray = this.layerSettingsControl;
    layerArray.clear();

    this.settings.layer_settings.forEach(layer => {
      layerArray.push(
        this.formBuilder.group({
          layer_name: [layer?.layer_name || "", Validators.required],
          model_type: [layer?.model_type || "", Validators.required]
        })
      );
    });

    // Model Provider
    const modelProviderSettings: IModelProviderSetting = this.settings.model_provider_settings;
    this.settingsForm.patchValue({
      model_provider_settings: {
        claude_settings: modelProviderSettings.claude_settings || this.defaultIndividualModelProviderSettings,
        deepseek_settings: modelProviderSettings.deepseek_settings || this.defaultIndividualModelProviderSettings,
        google_vertex_ai_settings: modelProviderSettings.google_vertex_ai_settings || this.defaultIndividualModelProviderSettings,
        grok_settings: modelProviderSettings.grok_settings || this.defaultIndividualModelProviderSettings,
        groq_settings: modelProviderSettings.groq_settings || this.defaultIndividualModelProviderSettings,
        ollama_settings: modelProviderSettings.ollama_settings || this.defaultIndividualModelProviderSettings,
        openai_settings: modelProviderSettings.openai_settings || this.defaultIndividualModelProviderSettings,
        three_d_model_type_settings: modelProviderSettings.three_d_model_type_settings || [],
        audio_model_type_settings: modelProviderSettings.audio_model_type_settings || [],
        image_model_type_settings: modelProviderSettings.image_model_type_settings || [],
        llm_model_type_settings: modelProviderSettings.llm_model_type_settings || [],
        rag_model_type_settings: modelProviderSettings.rag_model_type_settings || []
      }
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
    return this.settingsForm.get("ace_name");
  }

  get uiSettingsControl() {
    return this.settingsForm.get("ui_settings");
  }

  get layerSettingsControl(): FormArray {
    return this.settingsForm.get("layer_settings") as FormArray;
  }
}
