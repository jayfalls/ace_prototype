# DEPENDENCIES
## Local
from constants import (
    Defaults,
    LayerTypes,
    LLMModelTypes
)
from .layers import LayerSettings
from .model_providers import ModelProviderSettings, model_types
from .ui import UISettings


# UI
DEFAULT_UI_SETTINGS: UISettings = UISettings(
    dark_mode=True,
    show_footer=True
)


# MODEL PROVIDERS
## Model Types
DEFAULT_LLM_MODEL_TYPE_SETTINGS: list[model_types.LLMModelTypeSetting] = [
    model_types.LLMModelTypeSetting(
        model_type=LLMModelTypes.CODER,
        model_id="0194c745-de29-7bc6-ad6b-f7b5f8d0e414",
        creative_temperature=Defaults.CREATIVE_TEMPERATURE,
        logical_temperature=Defaults.LOGICAL_TEMPERATURE,
        output_token_limit=Defaults.OUTPUT_TOKEN_LIMIT
    ),
    model_types.LLMModelTypeSetting(
        model_type=LLMModelTypes.EFFICIENT,
        model_id="0194c74b-ff7d-7c91-a7b2-934a16aafb04",
        creative_temperature=Defaults.CREATIVE_TEMPERATURE,
        logical_temperature=Defaults.LOGICAL_TEMPERATURE,
        output_token_limit=Defaults.OUTPUT_TOKEN_LIMIT
    ),
    model_types.LLMModelTypeSetting(
        model_type=LLMModelTypes.FUNCTION_CALLER,
        model_id="0194c751-721a-7470-a277-e76ccdb840b8",
        creative_temperature=Defaults.CREATIVE_TEMPERATURE,
        logical_temperature=Defaults.LOGICAL_TEMPERATURE,
        output_token_limit=Defaults.OUTPUT_TOKEN_LIMIT
    ),
    model_types.LLMModelTypeSetting(
        model_type=LLMModelTypes.GENERALIST,
        model_id="0194c751-721a-7470-a277-e76ccdb840b8",
        creative_temperature=Defaults.CREATIVE_TEMPERATURE,
        logical_temperature=Defaults.LOGICAL_TEMPERATURE,
        output_token_limit=Defaults.OUTPUT_TOKEN_LIMIT
    ),
    model_types.LLMModelTypeSetting(
        model_type=LLMModelTypes.REASONER,
        model_id="0194c74e-13ea-72d5-9b2d-a46dfa196784",
        creative_temperature=Defaults.CREATIVE_TEMPERATURE,
        logical_temperature=Defaults.LOGICAL_TEMPERATURE,
        output_token_limit=Defaults.OUTPUT_TOKEN_LIMIT
    )
]

DEFAULT_RAG_MODEL_TYPE_SETTINGS: list[model_types.RAGModelTypeSetting] = []
# model_types.LLMModelTypeSetting(
#     model_type=RAGModelTypes.EMBEDDER,
#     model_name="granite-embedding:30m",
#     creative_temperature=Defaults.CREATIVE_TEMPERATURE,
#     logical_temperature=Defaults.LOGICAL_TEMPERATURE,
#     output_token_limit=Defaults.OUTPUT_TOKEN_LIMIT
# ),

## Overall
DEFAULT_MODEL_PROVIDER_SETTINGS = ModelProviderSettings(
    llm_model_type_settings=DEFAULT_LLM_MODEL_TYPE_SETTINGS,
    rag_model_type_settings=DEFAULT_RAG_MODEL_TYPE_SETTINGS
)


# LAYERS
DEFAULT_LAYER_SETTINGS: list[LayerSettings] = [
    LayerSettings(
        layer_name=LayerTypes.ASPIRATIONAL,
        model_type=LLMModelTypes.REASONER
    ),
    LayerSettings(
        layer_name=LayerTypes.GLOBAL_STRATEGY,
        model_type=LLMModelTypes.REASONER
    ),
    LayerSettings(
        layer_name=LayerTypes.AGENT_MODEL,
        model_type=LLMModelTypes.GENERALIST
    ),
    LayerSettings(
        layer_name=LayerTypes.EXECUTIVE_FUNCTION,
        model_type=LLMModelTypes.GENERALIST
    ),
    LayerSettings(
        layer_name=LayerTypes.COGNITIVE_CONTROL,
        model_type=LLMModelTypes.EFFICIENT
    ),
    LayerSettings(
        layer_name=LayerTypes.TASK_PROSECUTION,
        model_type=LLMModelTypes.FUNCTION_CALLER
    )
]
