# DEPENDENCIES
## Local
from .base_enum import BaseEnum


class ModelProviders(BaseEnum):
    ANTHROPIC: str = "anthropic"
    DEEPSEEK: str = "deepseek"
    GOOGLE_VERTEX_AI: str = "google_vertex_ai"
    GROK: str = "grok"
    GROQ: str = "groq"
    OLLAMA: str = "ollama"
    OPENAI: str = "openai"
