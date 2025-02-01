# DEPENDENCIES
## Local
from .base_enum import BaseEnum


class Names(BaseEnum):
    ACE: str = "ace"
    IMAGE: str = ACE
    FULL_IMAGE: str = f"localhost/{IMAGE}:latest"
    NETWORK: str = f"{ACE}_network"
    VOLUME: str = "volume"
