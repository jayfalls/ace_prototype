# DEPENDENCIES
## Local
from logger import logger

def start_actions(component_type: str) -> None:
    logger.startup(f"Starting {component_type}...")
