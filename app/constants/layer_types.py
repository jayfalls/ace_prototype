# DEPENDENCIES
## Local
from .base_enum import BaseEnum
from .components import Components


class LayerTypes(BaseEnum):
    ASPIRATIONAL: str = Components.ASPIRATIONAL
    GLOBAL_STRATEGY: str = Components.GLOBAL_STRATEGY
    AGENT_MODEL: str = Components.AGENT_MODEL
    EXECUTIVE_FUNCTION: str = Components.EXECUTIVE_FUNCTION
    COGNITIVE_CONTROL: str = Components.COGNITIVE_CONTROL
    TASK_PROSECUTION: str = Components.TASK_PROSECUTION
