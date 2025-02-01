# DEPENDENCIES
## Local
from .base_enum import BaseEnum
from .files import Files
from .names import Names


class ShellCommands(BaseEnum):
    UPDATE: str = "git pull"
    # Containers
    ## Deployment
    CHECK_PODS: str = "podman pod ps"
    _DEPLOY_KUBE_COMMAND: str = "podman kube play"
    DEPLOY_CLUSTER: str = f"{_DEPLOY_KUBE_COMMAND} --network {Names.NETWORK} --replace {Files.USER_DEPLOYMENT_FILE}"
    STOP_CLUSTER: str = f"{_DEPLOY_KUBE_COMMAND} --network {Names.NETWORK} --down {Files.USER_DEPLOYMENT_FILE}"
    ## Images
    CHECK_IMAGES: str = "podman images"
    BUILD_IMAGE: str = f"podman build -t {Names.IMAGE}  -f {Files.CONTAINERFILE} ."
    CLEAR_OLD_IMAGES: str = "podman image prune --force"
    ## Network
    CHECK_NETWORK: str = "podman network ls"
    CREATE_NETWORK: str = f"podman network create {Names.NETWORK}"
