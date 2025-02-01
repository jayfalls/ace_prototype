#!/usr/bin/env python3.13

# DEPENDENCIES
## Built-in
import argparse
## Local
from constants import Components
from logger import logger


# ARGUMENTS
def _get_arguments() -> dict[str, bool]:
    parser = argparse.ArgumentParser()
    parser.add_argument(f"--{Components.CONTROLLER}", action="store_true")
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

    selected_compenent: str | None = None
    for component, is_flagged in arguments.items():
        if not is_flagged:
            continue
        if selected_compenent:
            logger.critical("You can only start one component at a time!")
            exit(1)
        selected_compenent = component

    if not selected_compenent:
        logger.critical("You must select a component to start!")
        exit(1)

    logger.startup(f"Starting {selected_compenent.replace('_', ' ').title()}...")
    # TODO: Add logic to start components
    from time import sleep
    while True:
        logger.info(f"{selected_compenent.replace('_', ' ').title()} is running...")
        sleep(60)


if __name__ == "__main__":
    run_component()
