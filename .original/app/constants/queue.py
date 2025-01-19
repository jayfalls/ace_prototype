"""
Queue constants for the ace_prototype.
"""


# DEPENDENCIES
## Local
from .components import ComponentTypes as Queues
from .default import BaseEnum


class QueueCommands(BaseEnum):
    """Enum"""
    START: str = "nats-server -js"

QUEUES: frozenset[str] = frozenset(
    {
        Queues.ASPIRATIONAL,
        Queues.GLOBAL_STRATEGY,
        Queues.AGENT_MODEL,
        Queues.EXECUTIVE_FUNCTION,
        Queues.COGNITIVE_CONTROL,
        Queues.TASK_PROSECUTION,
    }
)


# BUSSES
class BusKeys(BaseEnum):
    """Enum"""
    UP: str = "northbound"
    DOWN: str = "southbound"

BUSSES_DOWN: dict[str, str] = {
    Queues.CONTROLLER: Queues.ASPIRATIONAL,
    Queues.ASPIRATIONAL: Queues.GLOBAL_STRATEGY,
    Queues.GLOBAL_STRATEGY: Queues.AGENT_MODEL,
    Queues.AGENT_MODEL: Queues.EXECUTIVE_FUNCTION,
    Queues.EXECUTIVE_FUNCTION: Queues.COGNITIVE_CONTROL,
    Queues.COGNITIVE_CONTROL: Queues.TASK_PROSECUTION,
}

BUSSES_UP: dict[str, str] = {
    Queues.TASK_PROSECUTION: Queues.COGNITIVE_CONTROL,
    Queues.COGNITIVE_CONTROL: Queues.EXECUTIVE_FUNCTION,
    Queues.EXECUTIVE_FUNCTION: Queues.AGENT_MODEL,
    Queues.AGENT_MODEL: Queues.GLOBAL_STRATEGY,
    Queues.GLOBAL_STRATEGY: Queues.ASPIRATIONAL,
}
