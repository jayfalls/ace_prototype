"""
Container constants for the ace_prototype.
"""


# DEPENDENCIES
## Built-in
import os
## Local
from .default import BaseEnum
from .components import ComponentTypes
from .generic import ACE


_SETUP_FOLDER: str = "./setup"
_ORCHESTRATOR: str = "podman"


# IMAGE
ACE_IMAGE_NAME: str = "ace_prototype"
_FULL_ACE_IMAGE_NAME: str = f"localhost/{ACE_IMAGE_NAME}:latest"

class ImageCommands(BaseEnum):
    """Enum"""
    CHECK_IMAGES: str = f"{_ORCHESTRATOR} images"
    BUILD_IMAGE: str = f"{_ORCHESTRATOR} build -t {ACE_IMAGE_NAME}  -f {_SETUP_FOLDER}/Containerfile ."


# VOLUMES
_VOLUME: str = "volume"

class VolumePaths(BaseEnum):
    """Enum"""
    STORAGE: str = "storage"
    HOST: str = f"{os.getcwd()}/{STORAGE}"
    HOST_CONTROLLER: str = f"{HOST}/controller"
    HOST_LAYERS: str = f"{HOST}/layers"
    HOST_MODEL_PROVIDER: str = f"{HOST}/model_provider"
    HOST_OUTPUT: str = f"{HOST}/output"
    CONTAINER: str = f"/home/{ACE.LOWER_NAME}/{STORAGE}"
    CONTROLLER: str = f"{CONTAINER}/controller"
    LAYERS: str = f"{CONTAINER}/layers"
    MODEL_PROVIDER: str = f"{CONTAINER}/model_provider"
    OUTPUT: str = f"{CONTAINER}/output"

class DevVolumePaths(BaseEnum):
    """Enum"""
    STORAGE: str = "./.storage"
    CONTROLLER: str = f"{STORAGE}/controller"
    CONTROLLER_SETTINGS: str = f"{CONTROLLER}/settings"
    LAYERS: str = f"{STORAGE}/layers"
    MODEL_PROVIDER: str = f"{STORAGE}/model_provider"
    OUTPUT: str = f"{STORAGE}/output"

REQUIRED_STORAGE_PATHS: tuple[str, ...] = (
    VolumePaths.HOST_CONTROLLER,
    VolumePaths.HOST_LAYERS,
    VolumePaths.HOST_MODEL_PROVIDER,
    VolumePaths.HOST_OUTPUT
)

REQUIRED_DEV_STORAGE_PATHS: tuple[str, ...] = (
    DevVolumePaths.CONTROLLER,
    DevVolumePaths.LAYERS,
    DevVolumePaths.MODEL_PROVIDER,
    DevVolumePaths.OUTPUT
)

# NETWORK
ACE_NETWORK_NAME: str = f"{ACE.LOWER_NAME}_network"

class NetworkCommands(BaseEnum):
    """Enum"""
    CHECK_NETWORK: str = f"{_ORCHESTRATOR} network ls"
    CREATE_NETWORK: str = f"{_ORCHESTRATOR} network create {ACE_NETWORK_NAME}"


# DEPLOYMENT
class DeploymentFile(BaseEnum):
    """Enum"""
    PATH: str = f"{_SETUP_FOLDER}/deployment.yaml"
    USER_PATH: str = f"{_SETUP_FOLDER}/.user_deployment.yaml"

class DeploymentCommands(BaseEnum):
    """Enum"""
    CHECK: str = f"{_ORCHESTRATOR} pod ps"
    _DEPLOY_COMMAND: str = f"{_ORCHESTRATOR} kube play"
    DEPLOY: str = f"{_DEPLOY_COMMAND} --network {ACE_NETWORK_NAME} --replace {DeploymentFile.USER_PATH}"
    STOP: str = f"{_DEPLOY_COMMAND} --network {ACE_NETWORK_NAME} --down {DeploymentFile.USER_PATH}"

class ComponentPorts(BaseEnum):
    """Enum"""
    CONTROLLER: str = "2349"
    QUEUE: str = "4222"
    MODEL_PROVIDER: str = "4223"
    TELEMETRY: str = "4931"
    ACTIONS: str = "4932"
    ASPIRATIONAL: str = "4581"
    GLOBAL_STRATEGY: str = "4582"
    AGENT_MODEL: str = "4583"
    EXECUTIVE_FUNCTION: str = "4584"
    COGNITIVE_CONTROL: str = "4585"
    TASK_PROSECUTION: str = "4586"

DEPLOYMENT_REPLACE_KEYWORDS: dict[str, str] = {
    "{{ ace_pod_name }}": ACE.LOWER_NAME,
    "{{ ace_image_name }}": _FULL_ACE_IMAGE_NAME,
    "{{ start_command }}": """python3\n    - main.py\n    - -sb\n    - -ct""",
    "{{ controller_name }}": ComponentTypes.CONTROLLER,
    "{{ controller_port }}": ComponentPorts.CONTROLLER,
    "{{ queue_name }}": ComponentTypes.QUEUE,
    "{{ queue_port }}": ComponentPorts.QUEUE,
    "{{ model_provider_name }}": ComponentTypes.MODEL_PROVIDER,
    "{{ model_provider_port }}": ComponentPorts.MODEL_PROVIDER,
    "{{ telemetry_name }}": ComponentTypes.TELEMETRY,
    "{{ telemetry_port }}": ComponentPorts.TELEMETRY,
    "{{ actions_name }}": ComponentTypes.ACTIONS,
    "{{ actions_port }}": ComponentPorts.ACTIONS,
    "{{ aspirational_name }}": ComponentTypes.ASPIRATIONAL,
    "{{ aspirational_port }}": ComponentPorts.ASPIRATIONAL,
    "{{ global_strategy_name }}": ComponentTypes.GLOBAL_STRATEGY,
    "{{ global_strategy_port }}": ComponentPorts.GLOBAL_STRATEGY,
    "{{ agent_model_name }}": ComponentTypes.AGENT_MODEL,
    "{{ agent_model_port }}": ComponentPorts.AGENT_MODEL,
    "{{ executive_function_name }}": ComponentTypes.EXECUTIVE_FUNCTION,
    "{{ executive_function_port }}": ComponentPorts.EXECUTIVE_FUNCTION,
    "{{ cognitive_control_name }}": ComponentTypes.COGNITIVE_CONTROL,
    "{{ cognitive_control_port }}": ComponentPorts.COGNITIVE_CONTROL,
    "{{ task_prosecution_name }}": ComponentTypes.TASK_PROSECUTION,
    "{{ task_prosecution_port }}": ComponentPorts.TASK_PROSECUTION,
    "{{ controller_host_path }}": VolumePaths.HOST_CONTROLLER,
    "{{ controller_container_path }}": VolumePaths.CONTROLLER,
    "{{ controller_volume }}": f"{ACE.LOWER_NAME}_{ComponentTypes.CONTROLLER}_{_VOLUME}",
    "{{ layers_host_path }}": VolumePaths.HOST_LAYERS,
    "{{ layers_container_path }}": VolumePaths.LAYERS,
    "{{ layers_volume }}": f"{ACE.LOWER_NAME}_layers_{_VOLUME}",
    "{{ model_provider_host_path }}": VolumePaths.HOST_MODEL_PROVIDER,
    "{{ model_provider_container_path }}": VolumePaths.MODEL_PROVIDER,
    "{{ model_provider_volume }}": f"{ACE.LOWER_NAME}_model_provider_{_VOLUME}",
    "{{ output_host_path }}": VolumePaths.HOST_OUTPUT,
    "{{ output_container_path }}": VolumePaths.OUTPUT,
    "{{ output_volume }}": f"{ACE.LOWER_NAME}_output_{_VOLUME}"
}
