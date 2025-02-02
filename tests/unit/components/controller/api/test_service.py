# DEPENDENCIES
## Built-In
import os
import json
## Third-Party
import pytest
## Local
from app.models.config.controller import ControllerSettingsSchema
from app.models.config.layers import LayerSettings
from app.models.config.model_providers import IndividualProviderSettings, ModelProviderSettings
from app.components.controller.api.service import (
    _get_settings,
    edit_settings_data
)
from app.constants import Files, DictKeys, LayerTypes, LLMModelTypes


# CONSTANTS
class ExistingSettings:
    ACE_NAME: str = "existing_name"
    LAYER_SETTINGS: list[dict] = [
        LayerSettings(
            layer_name=LayerTypes.ASPIRATIONAL,
            model_type=LLMModelTypes.EFFICIENT
        ).model_dump()
    ]
    MODEL_PROVIDER_SETTINGS = ModelProviderSettings(
        claude_settings=IndividualProviderSettings(enabled=True),
        llm_model_type_settings = [],
        rag_model_type_settings = []
    ).model_dump()


# HELPERS
def _empty_settings_file():
    """Sets the settings file to an empty dictionary"""
    with open(Files.CONTROLLER_SETTINGS, 'w') as f:
        json.dump({}, f)

def _default_settings_file():
    """Sets the settings file to an default values"""
    _empty_settings_file()
    _get_settings()

def _existing_settings_file():
    """Sets the settings file to an existing dictionary"""
    with open(Files.CONTROLLER_SETTINGS, 'w') as f:
        json.dump(
            ControllerSettingsSchema(
                ace_name=ExistingSettings.ACE_NAME,
                layer_settings=ExistingSettings.LAYER_SETTINGS,
                model_provider_settings=ExistingSettings.MODEL_PROVIDER_SETTINGS
            ).model_dump(),
            f
        )

def _assert_settings_populated():
    assert os.path.isfile(Files.CONTROLLER_SETTINGS), "Settings file should exist"
    with open(Files.CONTROLLER_SETTINGS, 'r') as f:
        settings = json.load(f)
        assert DictKeys.ACE_NAME in settings, f"Settings file should contain {DictKeys.ACE_NAME}"
        assert DictKeys.LAYER_SETTINGS in settings, f"Settings file should contain {DictKeys.LAYER_SETTINGS}"
        assert DictKeys.MODEL_PROVIDER_SETTINGS in settings, f"Settings file should contain {DictKeys.MODEL_PROVIDER_SETTINGS}"


# TESTS
def test_get_settings_populates_empty_file():
    """Test that _get_settings populates the settings file"""
    _empty_settings_file()
    settings: dict = _get_settings()
    assert isinstance(settings, dict), "Settings should be a dictionary"
    assert DictKeys.ACE_NAME in settings, f"Settings should contain {DictKeys.ACE_NAME}"
    assert DictKeys.LAYER_SETTINGS in settings, f"Settings should contain {DictKeys.LAYER_SETTINGS}"
    assert DictKeys.MODEL_PROVIDER_SETTINGS in settings, f"Settings should contain {DictKeys.MODEL_PROVIDER_SETTINGS}"
    _assert_settings_populated()

def test_get_settings_does_not_overwrite_existing_values():
    """Test that _get_settings does not overwrite existing settings"""
    _existing_settings_file()
    _get_settings()

    with open(Files.CONTROLLER_SETTINGS, 'r') as f:
        settings: dict = json.load(f)
        assert settings[DictKeys.ACE_NAME] == ExistingSettings.ACE_NAME, f"{DictKeys.ACE_NAME} should not be overwritten"
        assert settings[DictKeys.LAYER_SETTINGS] == ExistingSettings.LAYER_SETTINGS, f"{DictKeys.LAYER_SETTINGS} should not be overwritten"
        assert settings[DictKeys.MODEL_PROVIDER_SETTINGS] == ExistingSettings.MODEL_PROVIDER_SETTINGS, f"{DictKeys.MODEL_PROVIDER_SETTINGS} should not be overwritten"

def test_edit_settings_data():
    """Test that edit_settings_data updates the settings correctly."""
    updated_settings = {
        DictKeys.ACE_NAME: ExistingSettings.ACE_NAME,
        DictKeys.LAYER_SETTINGS: ExistingSettings.LAYER_SETTINGS,
        DictKeys.MODEL_PROVIDER_SETTINGS: ExistingSettings.MODEL_PROVIDER_SETTINGS
    }

    _empty_settings_file()
    edit_settings_data(updated_settings)
    
    with open(Files.CONTROLLER_SETTINGS, 'r') as f:
        new_settings: dict = json.load(f)
        assert new_settings[DictKeys.ACE_NAME] == ExistingSettings.ACE_NAME, f"{DictKeys.ACE_NAME} should be updated"
        assert new_settings[DictKeys.LAYER_SETTINGS] == ExistingSettings.LAYER_SETTINGS, f"{DictKeys.LAYER_SETTINGS} should be updated"
        assert new_settings[DictKeys.MODEL_PROVIDER_SETTINGS] == ExistingSettings.MODEL_PROVIDER_SETTINGS, f"{DictKeys.MODEL_PROVIDER_SETTINGS} should be updated"
