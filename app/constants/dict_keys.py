# DEPENDENCIES
# Local
from .base_enum import BaseEnum


class DictKeys(BaseEnum):
    DEV: str = "dev"
    BUILD: str = "build"
    FUNCTION_NAME: str = "function_name"
    LEVEL: str = "level"
    MESSAGE: str = "message"
    RESTART: str = "restart"
    STACKTRACE: str = "stacktrace"
    STOP: str = "stop"
    TIMESTAMP: str = "timestamp"
