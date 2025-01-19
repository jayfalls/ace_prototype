"""
Component constants for the ace_prototype.
"""

# DEPENDENCIES
## Local
from .default import BaseEnum

class ComponentTypes(BaseEnum):
    """Enum"""
    CONTROLLER: str = "controller"
    QUEUE: str = "queue"
    TELEMETRY: str = "telemetry"
    ACTIONS: str = "actions"
    MODEL_PROVIDER: str = "model_provider"
    ASPIRATIONAL: str = "aspirational"
    GLOBAL_STRATEGY: str = "global_strategy"
    AGENT_MODEL: str = "agent_model"
    EXECUTIVE_FUNCTION: str = "executive_function"
    COGNITIVE_CONTROL: str = "cognitive_control"
    TASK_PROSECUTION: str = "task_prosecution"
