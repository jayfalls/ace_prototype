# DEPENDENCIES
## Built-in
import os
import json
## Local
from .base_enum import BaseEnum
from .components import Components
from .container_folders import ContainerFolders
from .environment_variables import EnvironmentVariables
from .folders import Folders
from .names import Names
from .network import NetworkPorts


class Files(BaseEnum):
    """Enum"""
    # Containers
    CONTAINERFILE: str = f"{Folders.CONTAINERS}Containerfile"
    TEMPLATE_DEPLOYMENT_FILE: str = f"{Folders.CONTAINERS}template_deployment.yaml"
    USER_DEPLOYMENT_FILE: str = f"{Folders.CONTAINERS}.user_deployment.yaml"
    # Startup
    STARTUP_HISTORY: str = f"{Folders.STORAGE}.startup_history"
    VERSION: str = "version"
    # Storage
    CONTROLLER_SETTINGS: str = f"{Folders.CONTROLLER_STORAGE}.settings"
    CONTROLLER_THREE_D_MODELS: str = f"{Folders.CONTROLLER_MODEL_TYPES}.three_d_models"
    CONTROLLER_AUDIO_MODELS: str = f"{Folders.CONTROLLER_MODEL_TYPES}.audio_models"
    CONTROLLER_IMAGE_MODELS: str = f"{Folders.CONTROLLER_MODEL_TYPES}.image_models"
    CONTROLLER_LLM_MODELS: str = f"{Folders.CONTROLLER_MODEL_TYPES}.llm_models"
    CONTROLLER_MULTIMODAL_MODELS: str = f"{Folders.CONTROLLER_MODEL_TYPES}.multimodal_models"
    CONTROLLER_RAG_MODELS: str = f"{Folders.CONTROLLER_MODEL_TYPES}.rag_models"
    CONTROLLER_ROBOTICS_MODELS: str = f"{Folders.CONTROLLER_MODEL_TYPES}.robotics_models"
    CONTROLLER_VIDEO_MODELS: str = f"{Folders.CONTROLLER_MODEL_TYPES}.video_models"


# INIT
_ENSURE_JSON_FILES: frozenset[str] = frozenset([
    Files.STARTUP_HISTORY,
    Files.CONTROLLER_SETTINGS,
    Files.CONTROLLER_THREE_D_MODELS,
    Files.CONTROLLER_AUDIO_MODELS,
    Files.CONTROLLER_IMAGE_MODELS,
    Files.CONTROLLER_LLM_MODELS,
    Files.CONTROLLER_MULTIMODAL_MODELS,
    Files.CONTROLLER_RAG_MODELS,
    Files.CONTROLLER_ROBOTICS_MODELS,
    Files.CONTROLLER_VIDEO_MODELS
])
def _ensure_json_files():
    for file in _ENSURE_JSON_FILES:
        if not os.path.isfile(file):
            with open(file, "w", encoding="utf-8") as file:
                json.dump({}, file)
_ensure_json_files()

_DEPLOYMENT_REPLACE_KEYWORDS: dict[str, str] = {
    "{{ ace_pod_name }}": Names.ACE,
    "{{ ace_image_name }}": Names.FULL_IMAGE,
    "{{ start_command }}": """python3\n    - component.py\n    - --{{ env }}""",
    "{{ app_host_path }}": os.getcwd(),
    "{{ app_container_path }}": ContainerFolders.APP_DIR,
    "{{ app_volume }}": f"{Names.ACE}_app_{Names.VOLUME}",
    "{{ logs_host_path }}": Folders.HOST_LOGS,
    "{{ logs_container_path }}": ContainerFolders.LOGS,
    "{{ logs_volume }}": f"{Names.ACE}_logs_{Names.VOLUME}",
    "{{ logger_name_env }}": EnvironmentVariables.LOG_FILE_NAME,
    "{{ logger_verbose_env }}": EnvironmentVariables.LOGGER_VERBOSE,
    "{{ controller_name }}": Components.CONTROLLER,
    "{{ controller_port }}": NetworkPorts.CONTROLLER,
    "{{ ui_name }}": Components.UI,
    "{{ ui_port }}": NetworkPorts.UI,
    "{{ queue_name }}": Components.QUEUE,
    "{{ queue_port }}": NetworkPorts.QUEUE,
    "{{ model_provider_name }}": Components.MODEL_PROVIDER,
    "{{ model_provider_port }}": NetworkPorts.MODEL_PROVIDER,
    "{{ telemetry_name }}": Components.TELEMETRY,
    "{{ telemetry_port }}": NetworkPorts.TELEMETRY,
    "{{ actions_name }}": Components.ACTIONS,
    "{{ actions_port }}": NetworkPorts.ACTIONS,
    "{{ memory_name }}": Components.MEMORY,
    "{{ memory_port }}": NetworkPorts.MEMORY,
    "{{ aspirational_name }}": Components.ASPIRATIONAL,
    "{{ aspirational_port }}": NetworkPorts.ASPIRATIONAL,
    "{{ global_strategy_name }}": Components.GLOBAL_STRATEGY,
    "{{ global_strategy_port }}": NetworkPorts.GLOBAL_STRATEGY,
    "{{ agent_model_name }}": Components.AGENT_MODEL,
    "{{ agent_model_port }}": NetworkPorts.AGENT_MODEL,
    "{{ executive_function_name }}": Components.EXECUTIVE_FUNCTION,
    "{{ executive_function_port }}": NetworkPorts.EXECUTIVE_FUNCTION,
    "{{ cognitive_control_name }}": Components.COGNITIVE_CONTROL,
    "{{ cognitive_control_port }}": NetworkPorts.COGNITIVE_CONTROL,
    "{{ task_prosecution_name }}": Components.TASK_PROSECUTION,
    "{{ task_prosecution_port }}": NetworkPorts.TASK_PROSECUTION,
    "{{ controller_host_path }}": Folders.CONTROLLER_STORAGE,
    "{{ controller_container_path }}": ContainerFolders.CONTROLLER_STORAGE,
    "{{ controller_volume }}": f"{Names.ACE}_{Components.CONTROLLER}_{Names.VOLUME}",
    "{{ output_host_path }}": Folders.OUTPUT_STORAGE,
    "{{ output_container_path }}": ContainerFolders.OUTPUT_STORAGE,
    "{{ output_volume }}": f"{Names.ACE}_output_{Names.VOLUME}"
}
def setup_user_deployment_file(dev: bool):
    """Sets up the user deployment file"""
    if os.path.isfile(Files.USER_DEPLOYMENT_FILE):
        os.remove(Files.USER_DEPLOYMENT_FILE)
    with open(Files.TEMPLATE_DEPLOYMENT_FILE, "r", encoding="utf-8") as template_deployment_file:
        deployment_string: str = template_deployment_file.read()
        template_deployment_file.close()
    dev_env: str = "" if dev else "."
    deployment_string = deployment_string.replace("{{ dev_env }}", dev_env)
    for key, replace_string in _DEPLOYMENT_REPLACE_KEYWORDS.items():
        if key == "{{ start_command }}":
            replace_string = replace_string.replace("{{ env }}", "dev" if dev else "prod")
        deployment_string = deployment_string.replace(key, replace_string)
    with open(Files.USER_DEPLOYMENT_FILE, "w", encoding="utf-8") as user_deployment_file:
        user_deployment_file.write(deployment_string)
        user_deployment_file.close()
