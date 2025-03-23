# DEPENDENCIES
## Built-In
import json
## Local
from constants import (
    Files,
    ModelTypes, ThreeDModelTypes, AudioModelTypes, ImageModelTypes, LLMModelTypes, MultiModalModelTypes, RAGModelTypes, RoboticsModelTypes, VideoModelTypes
)
from models.data.initial import INTITAL_LLM_MODEL_PROVIDERS


def get_llm_model_types() -> tuple[str, ...]:
    return LLMModelTypes.get_tuple()

def get_llm_models() -> list[dict]:
    llm_model_providers: list[dict] = [initial_llm_model_provider.model_dump() for initial_llm_model_provider in INTITAL_LLM_MODEL_PROVIDERS]
    with open(Files.CONTROLLER_LLM_MODELS, "r", encoding="utf-8") as llm_models_file:
        llm_models: dict = json.loads(llm_models_file.read())
    llm_model_providers.extend(llm_models)
    return llm_model_providers
