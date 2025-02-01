# DEPENDENCIES
## Local
from logger import logger

def start_memory(component_type: str) -> None:
    logger.startup(f"Starting {component_type}...")
