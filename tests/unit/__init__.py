# DEPENDENCIES
## Built-In
import os
## Local
from app.constants.logger import EnvironmentVariables as LoggerEnvironmentVariables
from tests.unit.constants import TestingLoggerDefaults


# LOGGER FIX
os.environ[LoggerEnvironmentVariables.LOG_FILE_NAME] = TestingLoggerDefaults.LOG_FILE_NAME
os.environ[LoggerEnvironmentVariables.LOGGER_VERBOSE] = TestingLoggerDefaults.VERBOSE_ENABLED
