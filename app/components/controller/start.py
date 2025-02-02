# DEPENDENCIES
## Third-Party
import uvicorn
## Local
from constants import NetworkPorts
from logger import logger
from .api import controller_api

def start_controller(component_type: str, dev: bool) -> None:
    logger.startup(f"Starting {component_type}...")
    uvicorn.run(controller_api, host="0.0.0.0", port=int(NetworkPorts.CONTROLLER))
