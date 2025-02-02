# DEPENDENCIES
## Local
from logger import logger
from .config.defaults import DEFAULT_LLM_MODEL_TYPE_SETTINGS
from .data.required import REQUIRED_LLM_MODEL_TYPES


# TODO: Orchestrate data checks, validation and population here

def _initialise_database_with_defaults():
    # TODO: Implement this
    pass

def _verify_required_model_types():
    def _verify_required_model_type(category: str, default_settings: list, required_types: frozenset[str]):
        verified_types: list[str] = []
        for type_setting in default_settings:
            if type_setting.model_type in verified_types:
                continue
            if type_setting.model_type in required_types:
                verified_types.append(type_setting.model_type)
        if len(verified_types) != len(required_types):
            raise ValueError(f"Missing required LLM types: {required_types - set(verified_types)}")
    
    _verify_required_model_type(
        category="LLM",
        default_settings=DEFAULT_LLM_MODEL_TYPE_SETTINGS,
        required_types=REQUIRED_LLM_MODEL_TYPES
    )

def initialise():
    _initialise_database_with_defaults()
    _verify_required_model_types()
