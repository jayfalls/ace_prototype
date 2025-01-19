"""
Model Provider constants for the ace_prototype.
"""

# DEPENDENCIES
## Local
from .containers import VolumePaths
from .default import BaseEnum


class LLMKeys(BaseEnum):
    """Enum"""
    MODEL_TYPE: str = "model_type"
    PROVIDER_TYPE: str = "provider_type"

    # Config
    BASE_INFORMATION: str = "base_information"
    CURRENT_MAPPING: str = "current_mapping"

    # Provider Details
    API_KEY: str = "api_key"
    MODEL: str = "model"
    CONTEXT: str = "context"
    TEMPERATURE: str = "temperature"
    RATE_LIMIT: str = "rate_limit"
    LOW_VRAM: str = "low_vram"

class ModelProviderPaths(BaseEnum):
    """Enum"""
    CONFIG: str = f"{VolumePaths.HOST_MODEL_PROVIDER}/.config"

class LLMStackTypes(BaseEnum):
    """Enum"""
    GENERALIST: str = "generalist"
    EFFICIENT: str = "efficient"
    CODER: str = "coder"
    FUNCTION_CALLER: str = "function_caller"
    EMBEDDER: str = "embedder"
    RERANKER: str = "reranker"

class ModelTypes(BaseEnum):
    """Enum"""
    LLM: str = "llm"
    EMBEDDER: str = "embedder"
    RERANKER: str = "reranker"


# PROVIDERS
class Providers(BaseEnum):
    """Enum"""
    CLAUDE: str = "claude"
    GROQ: str = "groq"
    OLLAMA: str = "ollama"
    OPENAI: str = "openai"
    FAST_EMBED: str = "fast_embed"
    RAGATOUILLE: str = "ragatouille"
    CROSS_ENCODER: str = "cross_encoder"

class ClaudeModels(BaseEnum):
    """Enum"""
    OPUS: str = "claude-3-opus-20240229"
    SONNET: str = "claude-3-sonnet-20240229"
    HAIKU: str = "claude-3-haiku-20240307"

class GroqModels(BaseEnum):
    """Enum"""
    MIXTRAL: str = "mixtral-8x7b-32768"

class OllamaModels(BaseEnum):
    """Enum"""
    ALPHAMONARCH: str = "alphamonarch"
    PHI_TWO_ORANGE: str = "phi2-orange"
    STABLELM_TWO_ZEPHYR: str = "stablelm2:1.6b-zephyr-q6_K"
    DEEPSEEK_CODER: str = "deepseek-coder:6.7b-instruct-q3_K_L"
    DEEPSEEK_CODER_SMALL: str = "deepseek-coder:1.3b-instruct-q6_K"
    GORILLA_OPENFUNCTIONS: str = "adrienbrault/gorilla-openfunctions-v2:Q3_K_L"

class OpenAIModels(BaseEnum):
    """Enum"""
    FOUR: str = "gpt-4"
    FOUR_TURBO: str = "gpt-4-turbo-preview"
    THREE_POINT_FIVE: str = "gpt-3.5-turbo"

class RagatouilleModels(BaseEnum):
    """Enum"""
    MXBAI_COLBERT: str = "mixedbread-ai/mxbai-colbert-large-v1"

class FastEmbedModels(BaseEnum):
    """Enum"""
    MXBAI_EMBED: str = "mixedbread-ai/mxbai-embed-large-v1"

class CrossEncoderModels(BaseEnum):
    """Enum"""
    MXBAI_RERANKER: str = "mixedbread-ai/mxbai-rerank-large-v1"
