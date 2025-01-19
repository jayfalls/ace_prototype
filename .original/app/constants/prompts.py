"""
Prompt constants for the ace_prototype.
"""


# DEPENDENCIES
## Local
from .generic import Paths
from .default import BaseEnum


class PromptKeys(BaseEnum):
    TYPE: str = "prompt_type"
    PROMPT: str = "prompt"
    CONTEXT: str = "context"
    IDENTITY: str = "identity"
    MISSION: str = "ace_mission"
    GUIDANCE: str = "guidance"
    DATA: str = "data"
    TELEMETRY: str = "telemetry"
    RESPONSE_FORMAT: str = "response_format"
    SCHEMA: str = "schema"

class PromptTypes(BaseEnum):
    """Enum"""
    REFINE: str = "refine"
    OUTPUT: str = "output"

class PromptFilePaths(BaseEnum):
    """Enum"""
    _SYSTEM_PROMPTS: str = f"{Paths.LAYER}/system_prompts"
    LAYER: str = f"{_SYSTEM_PROMPTS}/layer"
    CONTEXT: str = f"{_SYSTEM_PROMPTS}/ace_context"
    IDENTITIES: str = f"{_SYSTEM_PROMPTS}/identities"
    _RESPONSE_SCHEMAS: str = f"{_SYSTEM_PROMPTS}/responses"
    ACTION_RESPONSE: str = f"{_RESPONSE_SCHEMAS}/action_response"
    OUTPUT_RESPONSE: str = f"{_RESPONSE_SCHEMAS}/output_response"
    RESPONSE_FORMAT: str = f"{_RESPONSE_SCHEMAS}/response_format"
    EXTRA_RULES: str = f"{_RESPONSE_SCHEMAS}/extra_rules"
    SCHEMAS: str = f"{_RESPONSE_SCHEMAS}/schemas"
