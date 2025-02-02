# DEPENDENCIES
## Built-In
import os
## Local
from app.constants import EnvironmentVariables
from tests.unit.testing_constants import TestingConfigs


# LOGGER FIX
os.environ[EnvironmentVariables.LOG_FILE_NAME] = TestingConfigs.LOG_FILE_NAME
os.environ[EnvironmentVariables.LOGGER_VERBOSE] = TestingConfigs.VERBOSE_ENABLED
