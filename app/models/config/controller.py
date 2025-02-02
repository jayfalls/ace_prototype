# DEPENDENCIES
## Third-Party
from pydantic import BaseModel
## Local
from constants import Defaults
from .defaults import DEFAULT_LAYER_SETTINGS, DEFAULT_MODEL_PROVIDER_SETTINGS
from .layers import LayerSettings
from .model_providers import ModelProviderSettings


class ControllerSettingsSchema(BaseModel):
    ace_name: str = Defaults.ACE_NAME
    layer_settings: list[LayerSettings] = DEFAULT_LAYER_SETTINGS
    model_provider_settings: ModelProviderSettings = DEFAULT_MODEL_PROVIDER_SETTINGS
