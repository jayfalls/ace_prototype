# DEPENDENCIES
## Built-In
from typing import Callable
## Local
from constants.components import ComponentTypes


# SHARED
from .queue import broker


# STARTUP
from .controller import component as _controller
from .queue.component import component as _queue
from .telemetry import _telemetry
from .actions import _actions
from .model_provider import _model_provider
from .layer import _layer

COMPONENT_MAP: dict[str, Callable[[str], None]] = {
    ComponentTypes.CONTROLLER: _controller,
    ComponentTypes.QUEUE: _queue,
    ComponentTypes.TELEMETRY: _telemetry,
    ComponentTypes.ACTIONS: _actions,
    ComponentTypes.MODEL_PROVIDER: _model_provider,
    ComponentTypes.ASPIRATIONAL: _layer,
    ComponentTypes.GLOBAL_STRATEGY: _layer,
    ComponentTypes.AGENT_MODEL: _layer,
    ComponentTypes.EXECUTIVE_FUNCTION: _layer,
    ComponentTypes.COGNITIVE_CONTROL: _layer,
    ComponentTypes.TASK_PROSECUTION: _layer,
}
