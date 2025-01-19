"""
Generic constants for the ace_prototype.
"""

# DEPENDENCIES
## Built-In
from typing import Any
## Local
from .default import BaseEnum


class ACE(BaseEnum):
    """Enum"""
    NAME: str = "ACE"
    LOWER_NAME: str = "ace"

class Paths(BaseEnum):
    COMPONENTS: str = "./components"
    ACTIONS: str = f"{COMPONENTS}/actions"
    CONTROLLER: str = f"{COMPONENTS}/controller"
    INPUTS: str = f"{COMPONENTS}/inputs"
    LAYER: str = f"{COMPONENTS}/layer"
    MEMORY: str = f"{COMPONENTS}/memory"
    MODEL_PROVIDER: str = f"{COMPONENTS}/model_provider"
    QUEUE: str = f"{COMPONENTS}/queue"
    
class GenericKeys(BaseEnum):
    """Enum"""
    NONE: str = "none"
    EMPTY: str = ""
    EMPTY_PATH: str = "./empty"
    DEFAULT: str = "default"


# TYPES
TOMLConfig = dict[str, dict[str, Any]]
