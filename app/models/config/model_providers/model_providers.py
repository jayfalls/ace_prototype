# DEPENDENCIES
## Third-Party
from pydantic import BaseModel
## Local
from constants import ModelProviders
from .individual_providers import IndividualProviderSettings
from . import model_types


class ModelProviderSettings(BaseModel):
    individual_provider_settings: list[IndividualProviderSettings] = [
        IndividualProviderSettings(
            name=ModelProviders.ANTHROPIC,
            enabled=False,
            api_key=""
        ),
        IndividualProviderSettings(
            name=ModelProviders.DEEPSEEK,
            enabled=False,
            api_key=""
        ),
        IndividualProviderSettings(
            name=ModelProviders.GOOGLE_VERTEX_AI,
            enabled=False,
            api_key=""
        ),
        IndividualProviderSettings(
            name=ModelProviders.GROK,
            enabled=False,
            api_key=""
        ),
        IndividualProviderSettings(
            name=ModelProviders.GROQ,
            enabled=False,
            api_key=""
        ),
        IndividualProviderSettings(
            name=ModelProviders.OLLAMA,
            enabled=False,
            api_key=""
        ),
        IndividualProviderSettings(
            name=ModelProviders.OPENAI,
            enabled=False,
            api_key=""
        )
    ]

    three_d_model_type_settings: list[model_types.ThreeDModelTypeSetting] = []
    audio_model_type_settings: list[model_types.AudioModelTypeSetting] = []
    image_model_type_settings: list[model_types.ImageModelTypeSetting] = []
    llm_model_type_settings: list[model_types.LLMModelTypeSetting]
    multimodal_model_type_settings: list[model_types.MultiModalModelTypeSetting] = []
    rag_model_type_settings: list[model_types.RAGModelTypeSetting]
    robotics_model_type_settings: list[model_types.RoboticsModelTypeSetting] = []
    video_model_type_settings: list[model_types.VideoModelTypeSetting] = []
