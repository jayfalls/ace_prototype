# DEPENDENCIES
## Local
from .base_enum import BaseEnum


class Defaults(BaseEnum):
    # API
    INTERNAL_SERVER_ERROR_MESSAGE: str = "Server experienced an internal error!"
    # Layers
    ACE_NAME: str = "PrototypeACE"
    # Model Provider
    CREATIVE_TEMPERATURE: float = 0.7
    LOGICAL_TEMPERATURE: float = 0.2
    OUTPUT_TOKEN_LIMIT: int = 2048
    # Logger
    TERMINAL_COLOR_CODE: str = "\033[0m"  # Default color
    SHUTDOWN_MESSAGE: str = "Shutting down logger..."
