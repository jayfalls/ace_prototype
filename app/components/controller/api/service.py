# DEPENDENCIES
## Built-In
import json
## Local
from constants import DictKeys, Files, ModelProviders


# HELPERS
def _get_settings() -> dict:
    settings: dict = {}
    with open(Files.CONTROLLER_SETTINGS, "r", encoding="utf-8") as settings_file:
        settings = json.loads(settings_file.read())
    return settings


# ROUTES
def get_settings_data() -> dict:
    return _get_settings()

def edit_settings_data(updated_settings: dict):
    settings: dict = _get_settings()
    new_model_provider: str | None = updated_settings.get(DictKeys.MODEL_PROVIDER)
    if new_model_provider:
        available_model_providers: dict = ModelProviders.get_frozenset()
        if new_model_provider not in available_model_providers:
            raise ValueError(f"Invalid model provider: {new_model_provider}")
    settings.update(updated_settings)
    with open(Files.CONTROLLER_SETTINGS, "w", encoding="utf-8") as settings_file:
        settings_file.write(json.dumps(settings))

def get_version_data() -> dict:
    with open(Files.VERSION, "r", encoding="utf-8") as settings_file:
        return json.loads(settings_file.read())
    