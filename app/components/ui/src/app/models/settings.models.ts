// SECTIONS
//// Layers
export interface ILayerSetting {
  layer_name: string;
  model_type: string;
}

//// Model Provider
////// Unique Providers
export interface IIndividualProviderSettings {
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
  claude_settings: IIndividualProviderSettings;
  deepseek_settings: IIndividualProviderSettings;
  google_vertex_ai_settings: IIndividualProviderSettings;
  grok_settings: IIndividualProviderSettings;
  groq_settings: IIndividualProviderSettings;
  ollama_settings: IIndividualProviderSettings;
  openai_settings: IIndividualProviderSettings;

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
  layer_settings: ILayerSetting[];
  model_provider_settings: IModelProviderSetting;
}
