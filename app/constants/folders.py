# DEPENDENCIES
## Built-In
import os
## Local
from .base_enum import BaseEnum


class Folders(BaseEnum):
    """Enum"""
    # Containers
    CONTAINERS: str = "containers/"
    # Logging
    LOGS: str = ".logs/"
    HOST_LOGS: str = f"{os.getcwd()}/{LOGS}"
    # Storage
    STORAGE: str = ".storage/"
    _HOST_STORAGE: str = f"{os.getcwd()}/{STORAGE}"
    CONTROLLER_STORAGE: str = f"{_HOST_STORAGE}controller/"
    CONTROLLER_MODEL_TYPES: str = f"{CONTROLLER_STORAGE}model_types/"
    OUTPUT_STORAGE: str = f"{_HOST_STORAGE}output/"


# INIT
_ENSURED_FOLDERS: tuple[str, ...] = (
    Folders.LOGS,
    Folders.STORAGE,
    Folders.CONTROLLER_STORAGE,
    Folders.CONTROLLER_MODEL_TYPES,
    Folders.OUTPUT_STORAGE
)

def _ensure_folders():
    for path in _ENSURED_FOLDERS:
        os.makedirs(path, exist_ok=True)
_ensure_folders()
