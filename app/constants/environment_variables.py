# DEPENDENCIES
# Local
from .base_enum import BaseEnum


class EnvironmentVariables(BaseEnum):
    LOG_FILE_NAME: str = "ACE_LOGGER_FILE_NAME"
    LOGGER_VERBOSE: str = "ACE_LOGGER_VERBOSE"
