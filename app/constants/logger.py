# DEPENDENCIES
## Built-In
import logging
import os


class CustomLogLevels:
    STARTUP: int = 25
    STACKTRACE: int = 45

COLOR_CODES: dict[int, str] = {
    logging.DEBUG: "\033[35m",    # Purple
    CustomLogLevels.STARTUP: "\033[36m",  # Cyan
    logging.INFO: "\033[34m",     # Blue
    logging.WARNING: "\033[33m",  # Yellow
    logging.ERROR: "\033[31m",    # Red
    CustomLogLevels.STACKTRACE: "\033[37m", # White
    logging.CRITICAL: "\033[30m", # Black
}

class Defaults:
    COLOR_CODE: str = "\033[0m"  # Default color
    LOG_FOLDER_NAME: str = "logs"
    LOG_FOLDER: str = os.path.abspath(LOG_FOLDER_NAME)
    SHUTDOWN_MESSAGE: str = "Shutting down logger..."

class DictKeys:
    FUNCTION_NAME: str = "function_name"
    LEVEL: str = "level"
    MESSAGE: str = "message"
    STACKTRACE: str = "stacktrace"
    TIMESTAMP: str = "timestamp"

class EnvironmentVariables:
    LOG_FILE_NAME: str = "ACE_LOGGER_FILE_NAME"
    LOGGER_VERBOSE: str = "ACE_LOGGER_VERBOSE"
