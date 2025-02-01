# DEPENDENCIES
## Local
from .base_enum import BaseEnum


class NetworkPorts(BaseEnum):
    """Enum"""
    CONTROLLER: str = "2349"
    UI: str = "2350"
    QUEUE: str = "4222"
    MODEL_PROVIDER: str = "4223"
    TELEMETRY: str = "4931"
    ACTIONS: str = "4932"
    MEMORY: str = "4933"
    ASPIRATIONAL: str = "4581"
    GLOBAL_STRATEGY: str = "4582"
    AGENT_MODEL: str = "4583"
    EXECUTIVE_FUNCTION: str = "4584"
    COGNITIVE_CONTROL: str = "4585"
    TASK_PROSECUTION: str = "4586"
