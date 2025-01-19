"""
Debug constants for the ace_prototype.
"""

# DEPENDENCIES
## Local
from .default import BaseEnum


class DebugLevels(BaseEnum):
    """Enum"""
    ERROR: int = 0
    WARNING: int = 1
    INFO: int = 2
    DEBUG: int = 3