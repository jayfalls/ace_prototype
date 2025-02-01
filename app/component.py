#!/usr/bin/env python3.13

# DEPENDENCIES
## Built-in
import argparse
from typing import Callable
## Local
from constants import Components, DictKeys
from logger import logger
from components import start_actions, start_controller, start_layer, start_memory, start_model_provider, start_queue, start_telemetry, start_ui


# CONSTANTS
_COMPONENT_MAP: dict[str, Callable[[str], None]] = {
    Components.CONTROLLER: start_controller,
    Components.UI: start_ui,
    Components.QUEUE: start_queue,
    Components.TELEMETRY: start_telemetry,
    Components.ACTIONS: start_actions,
    Components.MEMORY: start_memory,
    Components.MODEL_PROVIDER: start_model_provider,
    Components.ASPIRATIONAL: start_layer,
    Components.GLOBAL_STRATEGY: start_layer,
    Components.AGENT_MODEL: start_layer,
    Components.EXECUTIVE_FUNCTION: start_layer,
    Components.COGNITIVE_CONTROL: start_layer,
    Components.TASK_PROSECUTION: start_layer
}

# ARGUMENTS
def _get_arguments() -> dict[str, bool]:
    parser = argparse.ArgumentParser()
    parser.add_argument("--dev", action="store_true", help="Enable dev mode")
    parser.add_argument("--prod", action="store_true")
    parser.add_argument(f"--{Components.CONTROLLER}", action="store_true")
    parser.add_argument(f"--{Components.UI}", action="store_true")
    parser.add_argument(f"--{Components.QUEUE}", action="store_true")
    parser.add_argument(f"--{Components.TELEMETRY}", action="store_true")
    parser.add_argument(f"--{Components.ACTIONS}", action="store_true")
    parser.add_argument(f"--{Components.MEMORY}", action="store_true")
    parser.add_argument(f"--{Components.MODEL_PROVIDER}", action="store_true")
    parser.add_argument(f"--{Components.ASPIRATIONAL}", action="store_true")
    parser.add_argument(f"--{Components.GLOBAL_STRATEGY}", action="store_true")
    parser.add_argument(f"--{Components.AGENT_MODEL}", action="store_true")
    parser.add_argument(f"--{Components.EXECUTIVE_FUNCTION}", action="store_true")
    parser.add_argument(f"--{Components.COGNITIVE_CONTROL}", action="store_true")
    parser.add_argument(f"--{Components.TASK_PROSECUTION}", action="store_true")
    return vars(parser.parse_args())


# STARTUP
def run_component() -> None:
    """Startup the selected component"""
    arguments: dict[str, bool] = _get_arguments()
    
    dev: bool = arguments[DictKeys.DEV]
    prod: bool = arguments[DictKeys.PROD]
    if not (dev or prod):
        logger.critical("You must select a environment, either --dev or --prod!")
        exit(1)

    selected_compenent: str | None = None
    for argument, is_flagged in arguments.items():
        if argument == DictKeys.DEV or argument == DictKeys.PROD:
            continue
        if not is_flagged:
            continue
        if selected_compenent:
            logger.critical("You can only start one component at a time!")
            exit(1)
        selected_compenent = argument

    if not selected_compenent:
        logger.critical("You must select a component to start!")
        exit(1)
    
    if selected_compenent not in _COMPONENT_MAP:
        logger.critical(f"{selected_compenent} is not a valid component!")
        exit(1)

    component_title: str = selected_compenent.replace("_", " ").title()
    _COMPONENT_MAP[selected_compenent](component_type=component_title, dev=dev)
    from time import sleep
    while True:
        logger.info(f"{component_title} is running...")
        sleep(60)


if __name__ == "__main__":
    run_component()
