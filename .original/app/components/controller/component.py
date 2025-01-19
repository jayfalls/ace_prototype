# DEPENDENCIES
## Built-in
import os
from typing import Union
## Third-Party
import toml
import uvicorn
## Local
from .api import run
from constants.api import APIPaths
from constants.containers import DevVolumePaths, ComponentPorts


# SETUP
def setup(local_mode: bool) -> None:
    print("\nSetting Up Controller Files...")
    settings_path: str = APIPaths.SETTINGS
    if local_mode:
        dirs: tuple[str, ...] = (DevVolumePaths.CONTROLLER, DevVolumePaths.OUTPUT)
        _ = [os.makedirs(dirs, exist_ok=True) for dirs in dirs]
        settings_path = DevVolumePaths.CONTROLLER_SETTINGS
    if not os.path.exists(settings_path):
        open(settings_path, 'a').close()
    
    with open(APIPaths.RUNTIME_CONFIG, 'w', encoding='utf-8') as config_file:
        runtime_config: dict[str, Union[bool, str]] = {
            "local_mode": local_mode,
            "settings_path": settings_path
        }
        toml.dump({'runtime': runtime_config}, config_file)


# MAIN
def component(component_type: str) -> None:
    local_mode: bool = False
    setup(local_mode)
    print("\nStarting Server Client...")
    uvicorn.run(run.app, host="0.0.0.0", port=int(ComponentPorts.CONTROLLER))
