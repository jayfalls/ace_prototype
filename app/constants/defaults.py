# DEPENDENCIES
## Third-Party
from pydantic import BaseModel
## Local
from .base_enum import BaseEnum
from .model_providers import ModelProviders


class Defaults(BaseEnum):
    # API
    INTERNAL_SERVER_ERROR_MESSAGE: str = "Server experienced an internal error!"
    # Layers
    ACE_NAME: str = "PrototypeACE"
    # Model Provider
    MODEL_PROVIDER: str = ModelProviders.OLLAMA
    TEMPERATURE: float = 0.2
    # Logger
    TERMINAL_COLOR_CODE: str = "\033[0m"  # Default color
    SHUTDOWN_MESSAGE: str = "Shutting down logger..."

class DefaultAPIResponseSchema(BaseModel):
    message: str
