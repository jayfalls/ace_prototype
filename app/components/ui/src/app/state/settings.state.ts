import { createDefaultLoadable, Loadable } from "./loadable.state";
import { Values } from '../constants'
import { ISettings } from "../models/settings.models";

export interface SettingsState extends Loadable {
    settings: ISettings;
}

export function createInitialSettingsState(): SettingsState {
  return {
    ...createDefaultLoadable(),
    settings: {
      ace_name: Values.NOT_LOADED,
      layer_settings: [],
      model_provider_settings: {
        claude_settings: {
          api_key: Values.NOT_LOADED,
          enabled: false
        },
        deepseek_settings: {
          api_key: Values.NOT_LOADED,
          enabled: false
        },
        google_vertex_ai_settings: {
          api_key: Values.NOT_LOADED,
          enabled: false
        },
        grok_settings: {
          api_key: Values.NOT_LOADED,
          enabled: false
        },
        groq_settings: {
          api_key: Values.NOT_LOADED,
          enabled: false
        },
        ollama_settings: {
          api_key: Values.NOT_LOADED,
          enabled: false
        },
        openai_settings: {
          api_key: Values.NOT_LOADED,
          enabled: false
        },
        three_d_model_type_settings: [],
        audio_model_type_settings: [],
        image_model_type_settings: [],
        llm_model_type_settings: [
          {
            model_type: Values.NOT_LOADED,
            model_id: Values.NOT_LOADED,
            logical_temperature: 0,
            creative_temperature: 0,
            output_token_limit: 0
          }
        ],
        multimodal_model_type_settings: [],
        rag_model_type_settings: [],
        robotics_model_type_settings: [],
        video_model_type_settings: []
      }
    }
  }
}


