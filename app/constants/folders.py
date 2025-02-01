# DEPENDENCIES
## Built-In
import os
## Local
from .base_enum import BaseEnum


class Folders(BaseEnum):
    """Enum"""
    # Components
    _HOST_COMPONENTS: str = f"{os.getcwd()}/components/"
    UI: str = f"{_HOST_COMPONENTS}ui/"
    # Containers
    CONTAINERS: str = "containers/"
    # Logging
    LOGS: str = ".logs/"
    HOST_LOGS: str = f"{os.getcwd()}/{LOGS}"
    # Storage
    STORAGE: str = ".storage/"
    _HOST_STORAGE: str = f"{os.getcwd()}/{STORAGE}"
    CONTROLLER_STORAGE: str = f"{_HOST_STORAGE}controller"
    LAYERS_STORAGE: str = f"{_HOST_STORAGE}layers"
    MODEL_PROVIDER_STORAGE: str = f"{_HOST_STORAGE}model_provider"
    OUTPUT_STORAGE: str = f"{_HOST_STORAGE}output"


# INIT
_ENSURED_FOLDERS: tuple[str, ...] = (
    Folders.LOGS,
    Folders.STORAGE,
    Folders.CONTROLLER_STORAGE,
    Folders.LAYERS_STORAGE,
    Folders.MODEL_PROVIDER_STORAGE,
    Folders.OUTPUT_STORAGE
)

def _ensure_folders():
    for path in _ENSURED_FOLDERS:
        os.makedirs(path, exist_ok=True)
_ensure_folders()
