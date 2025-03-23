// SECTIONS
//// UI
export interface IUISettings {
  dark_mode: boolean;
  show_footer: boolean;
}

//// Layers
export interface ILayerSetting {
  layer_name: string;
  model_type: string;
}

//// Model Provider
////// Unique Providers
export interface IIndividualProviderSettings {
  name: string;
  enabled: boolean;
  api_key: string;
}

////// Model Types
export interface ILLMModelTypeSettings {
  model_type: string;
  model_id: string;
  logical_temperature: number;
  creative_temperature: number;
  output_token_limit: number;
}

////// Full
export interface IModelProviderSetting {
  individual_provider_settings: IIndividualProviderSettings[];

  three_d_model_type_settings: string[];
  audio_model_type_settings: string[];
  image_model_type_settings: string[];
  llm_model_type_settings: ILLMModelTypeSettings[];
  multimodal_model_type_settings: string[];
  rag_model_type_settings: string[];
  robotics_model_type_settings: string[];
  video_model_type_settings: string[];
}


// FULL SETTINGS
export interface ISettings {
  ace_name: string;
  ui_settings: IUISettings;
  layer_settings: ILayerSetting[];
  model_provider_settings: IModelProviderSetting;
}
