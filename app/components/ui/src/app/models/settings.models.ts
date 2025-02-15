// {
//   "ace_name": "PrototypeACE",
//   "layer_settings": [
//     {
//       "layer_name": "aspirational",
//       "model_type": "reasoner"
//     },
//     {
//       "layer_name": "global_strategy",
//       "model_type": "reasoner"
//     },
//     {
//       "layer_name": "agent_model",
//       "model_type": "generalist"
//     },
//     {
//       "layer_name": "executive_function",
//       "model_type": "generalist"
//     },
//     {
//       "layer_name": "cognitive_control",
//       "model_type": "efficient"
//     },
//     {
//       "layer_name": "task_prosecution",
//       "model_type": "function_caller"
//     }
//   ],
//   "model_provider_settings": {
//     "claude_settings": {
//       "enabled": false,
//       "api_key": ""
//     },
//     "deepseek_settings": {
//       "enabled": false,
//       "api_key": ""
//     },
//     "google_vertex_ai_settings": {
//       "enabled": false,
//       "api_key": ""
//     },
//     "grok_settings": {
//       "enabled": false,
//       "api_key": ""
//     },
//     "groq_settings": {
//       "enabled": false,
//       "api_key": ""
//     },
//     "ollama_settings": {
//       "enabled": true,
//       "api_key": ""
//     },
//     "openai_settings": {
//       "enabled": false,
//       "api_key": ""
//     },
//     "three_d_model_type_settings": [],
//     "audio_model_type_settings": [],
//     "image_model_type_settings": [],
//     "llm_model_type_settings": [
//       {
//         "model_type": "coder",
//         "model_id": "0194c745-de29-7bc6-ad6b-f7b5f8d0e414",
//         "logical_temperature": 0.2,
//         "creative_temperature": 0.7,
//         "output_token_limit": 2048
//       },
//       {
//         "model_type": "efficient",
//         "model_id": "0194c74b-ff7d-7c91-a7b2-934a16aafb04",
//         "logical_temperature": 0.2,
//         "creative_temperature": 0.7,
//         "output_token_limit": 2048
//       },
//       {
//         "model_type": "function_caller",
//         "model_id": "0194c751-721a-7470-a277-e76ccdb840b8",
//         "logical_temperature": 0.2,
//         "creative_temperature": 0.7,
//         "output_token_limit": 2048
//       },
//       {
//         "model_type": "generalist",
//         "model_id": "0194c751-721a-7470-a277-e76ccdb840b8",
//         "logical_temperature": 0.2,
//         "creative_temperature": 0.7,
//         "output_token_limit": 2048
//       },
//       {
//         "model_type": "reasoner",
//         "model_id": "0194c74e-13ea-72d5-9b2d-a46dfa196784",
//         "logical_temperature": 0.2,
//         "creative_temperature": 0.7,
//         "output_token_limit": 2048
//       }
//     ],
//     "multimodal_model_type_settings": [],
//     "rag_model_type_settings": [],
//     "robotics_model_type_settings": [],
//     "video_model_type_settings": []
//   }
// }

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
