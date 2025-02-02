# DEPENDENCIES
## Local
from constants import LLMModelTypes, RAGModelTypes


# MODEL PROVIDERS
REQUIRED_3D_MODEL_TYPES: frozenset[str] = frozenset()
REQUIRED_AUDIO_MODEL_TYPES: frozenset[str] = frozenset()
REQUIRED_IMAGE_MODEL_TYPES: frozenset[str] = frozenset()
REQUIRED_LLM_MODEL_TYPES: frozenset[str] = frozenset([
    LLMModelTypes.CODER,
    LLMModelTypes.EFFICIENT,
    LLMModelTypes.FUNCTION_CALLER,
    LLMModelTypes.GENERALIST,
    LLMModelTypes.REASONER
])
REQUIRED_MULTIMODAL_MODEL_TYPES: frozenset[str] = frozenset()
REQUIRED_RAG_MODEL_TYPES: frozenset[str] = frozenset([
    RAGModelTypes.EMBEDDER
])
REQUIRED_ROBOTICS_MODEL_TYPES: frozenset[str] = frozenset()
REQUIRED_VIDEO_MODEL_TYPES: frozenset[str] = frozenset()
