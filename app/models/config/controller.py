# DEPENDENCIES
## Third-Party
from pydantic import BaseModel
## Local
from constants import Defaults
from .defaults import DEFAULT_LAYER_SETTINGS, DEFAULT_MODEL_PROVIDER_SETTINGS, DEFAULT_UI_SETTINGS
from .layers import LayerSettings
from .model_providers import ModelProviderSettings
from .ui import UISettings


class ControllerSettingsSchema(BaseModel):
    ace_name: str = Defaults.ACE_NAME
    ui_settings: UISettings = DEFAULT_UI_SETTINGS
    model_provider_settings: ModelProviderSettings = DEFAULT_MODEL_PROVIDER_SETTINGS
    layer_settings: list[LayerSettings] = DEFAULT_LAYER_SETTINGS
