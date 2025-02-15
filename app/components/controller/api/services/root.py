# DEPENDENCIES
## Built-In
import json
## Local
from constants import Files
from models.config.controller import ControllerSettingsSchema


# HELPERS
def _get_settings() -> dict:
    settings: dict = {}
    with open(Files.CONTROLLER_SETTINGS, "r", encoding="utf-8") as settings_file:
        settings = json.loads(settings_file.read())
    settings = ControllerSettingsSchema(**settings).model_dump()
    with open(Files.CONTROLLER_SETTINGS, "w", encoding="utf-8") as settings_file:
        settings_file.write(json.dumps(settings))
    return settings


# ROOT
def get_version_data() -> dict:
    with open(Files.VERSION, "r", encoding="utf-8") as settings_file:
        return json.loads(settings_file.read())

def get_settings_data() -> dict:
    return _get_settings()

def edit_settings_data(updated_settings: dict):
    settings: dict = _get_settings()
    settings.update(updated_settings)
    with open(Files.CONTROLLER_SETTINGS, "w", encoding="utf-8") as settings_file:
        settings_file.write(json.dumps(settings))
