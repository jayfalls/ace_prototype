# DEPENDENCIES
## Local
from constants.layer import LayerKeys
from constants.prompts import PromptKeys, PromptFilePaths
from .inputs import (
    build_text_from_sub_layer_messages, get_telemetry
)
from .types import (
    InjectionMap, 
    FileInjection, ParamateriseFileInjection, MethodInjection, MethodInjectionParamaters, VariableInjection
)


# SINGLE INJECTIONS
## Base Prompt
_CONTEXT_FILE = FileInjection(
    replace_variable_name=PromptKeys.CONTEXT,
    load_file_path=PromptFilePaths.CONTEXT,
    sub_injection_map=None
)

_IDENTITY_FILE = ParamateriseFileInjection(
    replace_variable_name=PromptKeys.IDENTITY,
    load_file_folder=PromptFilePaths.IDENTITIES,
    load_file_variable_name=LayerKeys.TYPE,
    sub_injection_map=None
)

### Aspirational Base Prompt
_ASPIRATIONAL_MISSION_VARIABLE = VariableInjection(
    replace_variable_name=PromptKeys.MISSION
)

_ASPIRATIONAL_MISSION_MAP: InjectionMap = {
    PromptKeys.MISSION: _ASPIRATIONAL_MISSION_VARIABLE
}

_ASPIRATIONAL_IDENTITY_FILE = ParamateriseFileInjection(
    replace_variable_name=PromptKeys.IDENTITY,
    load_file_folder=PromptFilePaths.IDENTITIES,
    load_file_variable_name=LayerKeys.TYPE,
    sub_injection_map=_ASPIRATIONAL_MISSION_MAP
)

## Response
_GUIDANCE_METHOD = MethodInjection(
    replace_variable_name=PromptKeys.GUIDANCE,
    function=build_text_from_sub_layer_messages,
    paramaters=MethodInjectionParamaters(reference_variable_names=(PromptKeys.GUIDANCE,))
)

_DATA_METHOD = MethodInjection(
    replace_variable_name=PromptKeys.DATA,
    function=build_text_from_sub_layer_messages,
    paramaters=MethodInjectionParamaters(reference_variable_names=(PromptKeys.DATA,))
)

_TELEMETRY_METHOD = MethodInjection(
    replace_variable_name=PromptKeys.TELEMETRY,
    function=get_telemetry,
    paramaters=MethodInjectionParamaters(reference_variable_names=(PromptKeys.TELEMETRY, PromptKeys.CONTEXT))
)

_RESPONSE_SCHEMA_FILE = ParamateriseFileInjection(
    replace_variable_name=PromptKeys.SCHEMA,
    load_file_folder=PromptFilePaths.SCHEMAS,
    load_file_variable_name=LayerKeys.TYPE,
    sub_injection_map=None
)

_RESPONSE_FORMAT_MAP: InjectionMap = {
    PromptKeys.SCHEMA: _RESPONSE_SCHEMA_FILE
}

_RESPONSE_FORMAT_FILE = FileInjection(
    replace_variable_name=PromptKeys.RESPONSE_FORMAT,
    load_file_path=PromptFilePaths.RESPONSE_FORMAT,
    sub_injection_map=_RESPONSE_FORMAT_MAP
)

# INJECTION MAPS
BASE_PROMPT_MAP: InjectionMap = {
    PromptKeys.CONTEXT: _CONTEXT_FILE,
    PromptKeys.IDENTITY: _IDENTITY_FILE
}

ASPIRATIONAL_PROMPT_MAP: InjectionMap = BASE_PROMPT_MAP.copy()
ASPIRATIONAL_PROMPT_MAP[PromptKeys.IDENTITY] = _ASPIRATIONAL_IDENTITY_FILE

OUTPUT_RESPONSE_MAP: InjectionMap = {
    PromptKeys.GUIDANCE: _GUIDANCE_METHOD,
    PromptKeys.DATA: _DATA_METHOD,
    PromptKeys.TELEMETRY: _TELEMETRY_METHOD,
    PromptKeys.RESPONSE_FORMAT: _RESPONSE_FORMAT_FILE
}
