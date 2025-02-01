# DEPENDENCIES
## Built-In
import json
## Third-Party
import uvicorn
## Local
from constants import NetworkPorts
from logger import logger
from .api import controller_api, service
from .api.schemas import SettingsSchema
from constants import Files


def _ensure_settings() -> dict:
    settings: dict = service._get_settings()
    settings = SettingsSchema(**settings).dict()
    with open(Files.CONTROLLER_SETTINGS, "w", encoding="utf-8") as settings_file:
        settings_file.write(json.dumps(settings))

def start_controller(component_type: str, dev: bool) -> None:
    logger.startup(f"Starting {component_type}...")
    _ensure_settings()
    uvicorn.run(controller_api, host="0.0.0.0", port=int(NetworkPorts.CONTROLLER))
