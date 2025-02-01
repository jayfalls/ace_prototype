# DEPENDENCIES
## Local
from .base_enum import BaseEnum
from .folders import Folders


class ContainerFolders(BaseEnum):
    """Enum"""
    APP_DIR: str = "/home/ace/"
    # Logging
    LOGS: str = f"{APP_DIR}{Folders.LOGS}"
    # Storage
    STORAGE: str = f"{APP_DIR}{Folders.STORAGE}"
    CONTROLLER_STORAGE: str = f"{STORAGE}controller"
    LAYERS_STORAGE: str = f"{STORAGE}layers"
    MODEL_PROVIDER_STORAGE: str = f"{STORAGE}model_provider"
    OUTPUT_STORAGE: str = f"{STORAGE}output"
