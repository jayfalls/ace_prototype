# DEPENDENCIES
## Built-In
import json
## Local
from constants import (
    DictKeys,
    Files,
    ModelProviders,
    ModelTypes, ThreeDModelTypes, AudioModelTypes, ImageModelTypes, LLMModelTypes, MultiModalModelTypes, RAGModelTypes, RoboticsModelTypes, VideoModelTypes
)
from models.config.controller import ControllerSettingsSchema
from models.data.initial import INTITAL_LLM_MODEL_PROVIDERS


# HELPERS
def _get_settings() -> dict:
    settings: dict = {}
    with open(Files.CONTROLLER_SETTINGS, "r", encoding="utf-8") as settings_file:
        settings = json.loads(settings_file.read())
    settings = ControllerSettingsSchema(**settings).model_dump()
    with open(Files.CONTROLLER_SETTINGS, "w", encoding="utf-8") as settings_file:
        settings_file.write(json.dumps(settings))
    return settings


# GENERAL
def get_version_data() -> dict:
    with open(Files.VERSION, "r", encoding="utf-8") as settings_file:
        return json.loads(settings_file.read())

def get_settings_data() -> dict:
    return _get_settings()

def edit_settings_data(updated_settings: dict):
    settings: dict = _get_settings()
    settings.update(updated_settings)
    with open(Files.CONTROLLER_SETTINGS, "w", encoding="utf-8") as settings_file:
        settings_file.write(json.dumps(settings))

# MODEL PROVIDERS
## Model Types
def get_model_types() -> dict[str, tuple[str, ...]]:
    return {
        ModelTypes.THREE_D: ThreeDModelTypes.get_tuple(),
        ModelTypes.AUDIO: AudioModelTypes.get_tuple(),
        ModelTypes.IMAGE: ImageModelTypes.get_tuple(),
        ModelTypes.LLM: LLMModelTypes.get_tuple(),
        ModelTypes.MULTIMODAL: MultiModalModelTypes.get_tuple(),
        ModelTypes.RAG: RAGModelTypes.get_tuple(),
        ModelTypes.ROBOTICS: RoboticsModelTypes.get_tuple(),
        ModelTypes.VIDEO: VideoModelTypes.get_tuple()
    }

def get_llm_models() -> list[dict]:
    llm_model_providers: list[dict] = [initial_llm_model_provider.model_dump() for initial_llm_model_provider in INTITAL_LLM_MODEL_PROVIDERS]
    with open(Files.CONTROLLER_LLM_MODELS, "r", encoding="utf-8") as llm_models_file:
        llm_models: dict = json.loads(llm_models_file.read())
    llm_model_providers.extend(llm_models)
    return llm_model_providers
