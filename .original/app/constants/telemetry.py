"""
Layer constants for the ace_prototype.
"""

# DEPENDENCIES
## Local
from .default import BaseEnum
from .generic import GenericKeys


class TelemetryKeys(BaseEnum):
    """Enum"""
    WORLD_OVERVIEW_CACHE: str = "world_overview_cache"

class TelemetryPaths(BaseEnum):
    """Enum"""
    SYSTEM_PROMPTS: str = "components/telemetry/system_prompts"

class TelemetrySystemPrompts(BaseEnum):
    """Enum"""
    WORLD_OVERVIEW: str = f"{TelemetryPaths.SYSTEM_PROMPTS}/world_overview"

class TelemetryTypes(BaseEnum):
    """Enum"""
    NONE: str = GenericKeys.NONE
    TIME: str = "time"
    LOCATION: str = "location"
    EMBODIMENT: str = "embodiment"
    WORLD_OVERVIEW: str = "world_overview"
    HARDWARE_STATS: str = "hardware_statistics"
    SYSTEM_METRICS: str = "system_metrics"
    SOFTWARE_STATS: str = "software_statistics"
    SYSTEM_PROCESSES: str = "system_processes"
    RESOURCES: str = "resources"
    MEMORY: str = "memory"
    VISUAL: str = "visual"
    AUDIO: str = "audio"
    STDOUT: str = "stdout"
    USER_INPUT: str = "user_input"
