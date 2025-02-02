# DEPENDENCIES
# Local
from .base_enum import BaseEnum


class DictKeys(BaseEnum):
    ACE_NAME: str = "ace_name"
    DEV: str = "dev"
    BUILD: str = "build"
    FUNCTION_NAME: str = "function_name"
    LEVEL: str = "level"
    MESSAGE: str = "message"
    MODEL_PROVIDER: str = "model_provider"
    PROD: str = "prod"
    REBUILD_DATE: str = "rebuild_date"
    RESTART: str = "restart"
    STACKTRACE: str = "stacktrace"
    STOP: str = "stop"
    TEMPERATURE: str = "temperature"
    TIMESTAMP: str = "timestamp"
