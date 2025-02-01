# DEPENDENCIES
## Local
from .base_enum import BaseEnum


class Components(BaseEnum):
    """Enum"""
    CONTROLLER: str = "controller"
    UI: str = "ui"
    QUEUE: str = "queue"
    TELEMETRY: str = "telemetry"
    ACTIONS: str = "actions"
    MEMORY: str = "memory"
    MODEL_PROVIDER: str = "model_provider"
    ASPIRATIONAL: str = "aspirational"
    GLOBAL_STRATEGY: str = "global_strategy"
    AGENT_MODEL: str = "agent_model"
    EXECUTIVE_FUNCTION: str = "executive_function"
    COGNITIVE_CONTROL: str = "cognitive_control"
    TASK_PROSECUTION: str = "task_prosecution"
