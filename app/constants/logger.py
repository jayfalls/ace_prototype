# DEPENDENCIES
## Built-In
import logging
## Local
from .base_enum import BaseEnum


class CustomLogLevels(BaseEnum):
    STARTUP: int = 25
    STACKTRACE: int = 45

TERMINAL_COLOR_CODES: dict[int, str] = {
    logging.DEBUG: "\033[35m",    # Purple
    CustomLogLevels.STARTUP: "\033[36m",  # Cyan
    logging.INFO: "\033[34m",     # Blue
    logging.WARNING: "\033[33m",  # Yellow
    logging.ERROR: "\033[31m",    # Red
    CustomLogLevels.STACKTRACE: "\033[37m", # White
    logging.CRITICAL: "\033[30m", # Black
}
