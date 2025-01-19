"""
Layer constants for the ace_prototype.
"""

# DEPENDENCIES
## Local
from .containers import VolumePaths
from .components import ComponentTypes
from .default import BaseEnum
from .generic import GenericKeys


class LayerKeys(BaseEnum):
    """Enum"""
    NAME: str = "name"
    TYPE: str = "layer_type"

    # Bus
    QUEUE: str = "queue"
    MESSAGE_TYPE: str = "message_type"
    MESSAGES: str = "messages"
    HEADING: str = "heading"
    CONTENT: str = "content"

    # Config
    BASE_INFORMATION: str = "base_information"
    CURRENT_ACE: str = "current_ace"
    MISSION: str = "ace_mission"

    # Prompt Files
    BASE_PROMPT: str = "base_prompt"

    # State
    FIRST_RUN: str = "first_run"
    PROCESSING: str = "processing"
    MAX_RETRIES: str = "max_retries"
    DEFAULT_GUIDANCE: str = "default_guidance"
    HAS_DATA: str = "has_data"
    DEFAULT_DATA: str = "default_data"

    # Message Types
    COMMANDS: str = "commands"
    INTERNAL: str = "internal"
    GUIDANCE: str = "guidance"
    DATA: str = "data"
    TELEMETRY: str = "telemetry"

    # Sub Message Types
    ACTIONS: str = "actions"

class LayerPaths(BaseEnum):
    """Enum"""
    CONFIG: str = f"{VolumePaths.HOST_LAYERS}/.config"

class Layers(BaseEnum):
    """Enum"""
    ASPIRATIONAL: str = ComponentTypes.ASPIRATIONAL
    GLOBAL_STRATEGY: str = ComponentTypes.GLOBAL_STRATEGY
    AGENT_MODEL: str = ComponentTypes.AGENT_MODEL
    EXECUTIVE_FUNCTION: str = ComponentTypes.EXECUTIVE_FUNCTION
    COGNITIVE_CONTROL: str = ComponentTypes.COGNITIVE_CONTROL
    TASK_PROSECUTION: str = ComponentTypes.TASK_PROSECUTION

class LayerCommands(BaseEnum):
    """Enum"""
    NONE: str = GenericKeys.NONE
    POST: str = "power_on_self_test"


# CAPABILITIES
class ActionTags(BaseEnum):
    """Enum"""
    NONE: str = GenericKeys.NONE
    WARNING: str = "WARNING! This is a very expensive operation, use only when necessary..."
    OPTIONAL: str = "Don't show this feature if relevant flag is disabled..."

class Actions(BaseEnum):
    """Enum"""
    NONE: str = GenericKeys.NONE
    MATH: str = "math"
    WORKFLOWS: str = "workflows"
    FILE: str = "files"
    SHELL: str = "shell"
    DATABASE: str = "database"
    INTERNET: str = "internet"
    API: str = "api"
    SPEAK: str = "chat"
