# DEPENDENCIES
## Local
from logger import logger

def start_queue(component_type: str) -> None:
    logger.startup(f"Starting {component_type}...")
