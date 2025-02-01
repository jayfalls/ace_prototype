# DEPENDENCIES
## Local
from .base_enum import BaseEnum


class Defaults(BaseEnum):
    TERMINAL_COLOR_CODE: str = "\033[0m"  # Default color
    SHUTDOWN_MESSAGE: str = "Shutting down logger..."
