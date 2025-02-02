# DEPENDENCIES
## Third-Party
from pydantic import BaseModel
## Local
from .indiividual_providers import IndividualProviderSettings
from . import model_types


class ModelProviderSettings(BaseModel):
    claude_settings: IndividualProviderSettings = IndividualProviderSettings()
    deepseek_settings: IndividualProviderSettings = IndividualProviderSettings()
    google_vertex_ai_settings: IndividualProviderSettings = IndividualProviderSettings()
    grok_settings: IndividualProviderSettings = IndividualProviderSettings()
    groq_settings: IndividualProviderSettings = IndividualProviderSettings()
    ollama_settings: IndividualProviderSettings = IndividualProviderSettings(enabled=True)
    openai_settings: IndividualProviderSettings = IndividualProviderSettings()

    three_d_model_type_settings: list[model_types.ThreeDModelTypeSetting] = []
    audio_model_type_settings: list[model_types.AudioModelTypeSetting] = []
    image_model_type_settings: list[model_types.ImageModelTypeSetting] = []
    llm_model_type_settings: list[model_types.LLMModelTypeSetting]
    multimodal_model_type_settings: list[model_types.MultiModalModelTypeSetting] = []
    rag_model_type_settings: list[model_types.RAGModelTypeSetting]
    robotics_model_type_settings: list[model_types.RoboticsModelTypeSetting] = []
    video_model_type_settings: list[model_types.VideoModelTypeSetting] = []
