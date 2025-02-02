# DEPENDENCIES
## Local
from .base_enum import BaseEnum


# BASE
class ModelTypes(BaseEnum):
    THREE_D: str = "3d"
    AUDIO: str = "audio"
    IMAGE: str = "image"
    LLM: str = "llm"
    MULTIMODAL: str = "multimodal"
    RAG: str = "rag"
    ROBOTICS: str = "robotics"
    VIDEO: str = "video"


# INDIVIDUAL
class ThreeDModelTypes(BaseEnum):
    THREED_MODEL_GENERATOR: str = "3d_model_generator"

class AudioModelTypes(BaseEnum):
    AUDIO_GENERATOR: str = "audio_generator"
    AUDIO_TRANSCRIPTIONIST: str = "audio_transcriptionist"

class ImageModelTypes(BaseEnum):
    IMAGE_GENERATOR: str = "image_generator"

class LLMModelTypes(BaseEnum):
    CODER: str = "coder"
    EFFICIENT: str = "efficient"
    FUNCTION_CALLER: str = "function_caller"
    GENERALIST: str = "generalist"
    REASONER: str = "reasoner"

class MultiModalModelTypes(BaseEnum):
    AUDIO_ONLY_MULTIMODAL: str = "audio_only_multimodal"
    FULLY_MULTIMODAL: str = "fully_multimodal"
    IMAGE_ONLY_MULTIMODAL: str = "image_only_multimodal"

class RAGModelTypes(BaseEnum):
    EMBEDDER: str = "embedder"
    RERANKER: str = "reranker"

class RoboticsModelTypes(BaseEnum):
    ROBOTICS_CONTROLLER: str = "robotics_controller"

class VideoModelTypes(BaseEnum):
    VIDEO_GENERATOR: str = "video_generator"
