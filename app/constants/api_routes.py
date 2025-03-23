# DEPENDENCIES
## Local
from .base_enum import BaseEnum


class APIRoutes(BaseEnum):
    """Enum"""
    ROOT: str = "/"
    MODEL_PROVIDER: str = f"{ROOT}model-provider/"
