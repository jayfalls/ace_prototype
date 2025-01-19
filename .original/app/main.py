#!/usr/bin/env python3
"""
Starting point for the ACE Prototype, as well as the individual components.

Author: jayfalls

Arguments:
    -d/--dev: bool -> Enable dev mode
    -t/--test: bool -> Run tests
    -b/--build: bool -> Build the images
    -nb/--no-build: bool -> Skip the build check
    -s/--stop: bool -> Stop the ACE cluster
    -r/--restart: bool -> Restart the ACE cluster deployment
    -u/--update: bool -> Update the ACE
    -ct/--component-type: str -> The type of this component
"""


# DEPENDENCIES
## Built-in
from argparse import ArgumentParser
import os
from typing import Callable
## Third Party
import pytest
## Local
from constants.components import ComponentTypes
from constants.settings import DebugLevels
from constants.startup import StartupCommands
from constants.arguments import (ArgumentNames, ARGUMENTS, ARGUMENTS_HELP,
    BOOL_ARGUMENTS, STRING_ARGUMENTS, ARGUMENTS_SHORT
)
from constants.containers import (ACE_IMAGE_NAME, ImageCommands,
    REQUIRED_STORAGE_PATHS, REQUIRED_DEV_STORAGE_PATHS,
    ACE_NETWORK_NAME, NetworkCommands,
    DeploymentFile, DEPLOYMENT_REPLACE_KEYWORDS, DeploymentCommands
)
from constants.generic import ACE
from components import COMPONENT_MAP
from exceptions.error_handling import exit_on_error
from helpers import debug_print, execute, exec_check_exists


# VARIABLES
class ACEArguments:
    """
    Class representing arguments for the ACE.
    
    Attributes:
        dev (bool): Flag indicating dev mode.
        should_test (bool): Flag indicating if testing should be done.
        should_build (bool): Flag indicating if building should be done.
        no_build (bool): Flag indicating to skip building.
        local_mode (bool): Flag indicating local mode.
        stop (bool): Flag indicating to stop.
        should_restart (bool): Flag indicating if restart is needed.
        should_update (bool): Flag indicating if update is needed.
        component (str): String representing a component.
    """
    dev: bool = False
    should_test: bool = False
    should_build: bool = False
    no_build: bool = False
    stop: bool = False
    should_restart: bool = False
    should_update: bool = False
    component: str = ""


# ARGUMENTS
def _set_arguments(arg_parser: ArgumentParser) -> None:
    for argument in BOOL_ARGUMENTS:
        arg_parser.add_argument(*tuple(ARGUMENTS.get(argument, "")), action='store_true', required=False, help=ARGUMENTS_HELP[argument])
    for argument in STRING_ARGUMENTS:
        arg_parser.add_argument(*tuple(ARGUMENTS.get(argument, "")), type=str, required=False, help=ARGUMENTS_HELP[argument])

def _parse_arguments(arg_parser: ArgumentParser) -> ACEArguments:
    arguments: dict = {key: value for key, value in vars(arg_parser.parse_args()).items() if value is not None}
    ace_arguments = ACEArguments()
    ace_arguments.dev = arguments.get(ArgumentNames.DEV, False)
    if ace_arguments.dev:
        pass
        # ASSIGN DEV LOGIC
    ace_arguments.component = arguments.get(ArgumentNames.COMPONENT_TYPE, ArgumentNames.STARTUP)
    ace_arguments.should_test = arguments.get(ArgumentNames.TEST, False)
    ace_arguments.should_build = arguments.get(ArgumentNames.BUILD, False)
    ace_arguments.no_build = arguments.get(ArgumentNames.SKIP_BUILD, False)
    ace_arguments.stop = arguments.get(ArgumentNames.STOP, False)
    ace_arguments.should_update = arguments.get(ArgumentNames.UPDATE, False)
    ace_arguments.should_restart = arguments.get(ArgumentNames.RESTART, False)
    debug_print(f"Arguments: {ace_arguments.__dict__}", DebugLevels.DEBUG)
    if ace_arguments.component not in ComponentTypes.get_frozen_values() and ace_arguments.component != ArgumentNames.STARTUP:
        exit_on_error(f"Invalid component type: {ace_arguments.component}!\nPlease use one of the following: {ComponentTypes.get_values()}")
    return ace_arguments

def assign_arguments() -> ACEArguments:
    """
    Assigns runtime arguments.

    Arguments:
        None
    
    Returns:
        ACEArguments containing the runtime arguments.
    """
    arg_parser = ArgumentParser()
    _set_arguments(arg_parser)
    ace_arguments: ACEArguments = _parse_arguments(arg_parser)
    return ace_arguments


# SETUP
def _setup_folders() -> None:
    required_paths: tuple[str, ...] = (*REQUIRED_STORAGE_PATHS, *REQUIRED_DEV_STORAGE_PATHS)
    _ = [os.makedirs(dir_path, exist_ok=True) for dir_path in required_paths]

def _setup_user_deployment_file() -> None:
    if os.path.isfile(DeploymentFile.USER_PATH):
        return
    print("\nFirst time setting up user deployment file...")
    with open(DeploymentFile.PATH, "r", encoding="utf-8") as deployment_file:
        deployment_string: str = deployment_file.read()
        deployment_file.close()
    for key, replace in DEPLOYMENT_REPLACE_KEYWORDS.items():
        deployment_string = deployment_string.replace(key, replace)
    with open(DeploymentFile.USER_PATH, "w", encoding="utf-8") as user_deployment_file:
        user_deployment_file.write(deployment_string)
        user_deployment_file.close()
    # DO DEV AS WELL

def setup() -> None:
    """
    A function to set up folders and the user deployment file.
    
    Arguments:
        None
    
    Returns:
        None
    """
    _setup_folders()
    _setup_user_deployment_file()


# BUILD
def build(request_build: bool) -> bool:
    """
    A function which builds if image doesn't exist or if request_build flag is set.
    
    Arguments: 
        request_build: bool | Flag indicating if a build is requested.
    
    Returns:
        A bool indicating whether a build was performed.
    """
    print("\nChecking if build is required...")
    check_images_command: str = ImageCommands.CHECK_IMAGES
    image_exists: bool = exec_check_exists(check_images_command, ACE_IMAGE_NAME)
    should_build: bool = not image_exists or request_build
    if should_build:
        if not image_exists:
            print("\nImage does not exist\nBuilding container...")
        else:
            print("\nBuilding container...")
        build_ace_images_command: str = ImageCommands.BUILD_IMAGE
        execute(build_ace_images_command, error_message="Unable to build image")
        return should_build
    print("\nImage already exists\nSkipping build...")
    return should_build


# TESTS
def _run_tests(tests_to_run: tuple[str, ...]) -> None:
    print("Running tests...")
    # Create a pytest argument parser
    args = ['--capture=no']

    # Add the names of the tests to run to the arguments list
    args.extend(tests_to_run)

    # Run the tests using pytest's main function
    pytest.main(args)

def test() -> None:
    """
    Fake test.

    Arguments:
        None

    Returns:
        None
    """
    print("\nTest Mode\n")
    tests: tuple[str, ...] = ("test_1", "test_2", "test_3")
    _run_tests(tests)


# RUNTIME
def update() -> None:
    """
    Update the ACE.
    
    Arguments:
        None
    
    Returns:
        None
    """
    print(f"\nUpdating {ACE.NAME}...")
    update_command: str = StartupCommands.UPDATE
    execute(update_command)

def _setup_network() -> None:
    if not exec_check_exists(NetworkCommands.CHECK_NETWORK, ACE_NETWORK_NAME):
        print("\nFirst time setting up network...")
        execute(NetworkCommands.CREATE_NETWORK)

def stop(exists: bool) -> None:
    """
    Stops the ACE if it exists.

    Arguments:
        exists: bool | Indicates if the ace is running.
    
    Returns:
        None
    """
    if not exists:
        print(f"{ACE.NAME} is not running! Cannot stop...")
        return
    print(f"\nStopping {ACE.NAME}...")
    execute(DeploymentCommands.STOP)

def start_ace(restart: bool, exists: bool) -> None:
    """
    Start the ACE, handling restarts.

    Arguments:
        restart: bool | Whether to restart the ACE.
        exists: bool | Whether the ACE is already running.

    Returns:
        None
    """
    deploy_command: str = DeploymentCommands.DEPLOY
    if restart:
        print(f"\nRestarting {ACE.NAME}...")
        execute(deploy_command)
        return
    if exists:
        print(f"\nACE is already running...\n\nPlease run with {ARGUMENTS_SHORT[ArgumentNames.RESTART]} to restart!")
        return
    print(f"\nStarting {ACE.NAME}...")
    execute(deploy_command)

def start_component(component_type: str) -> None:
    """
    Start a component of a specified type, optionally in dev mode.

    Args:
        component_type: str | The type of component to start.
        dev: bool | Whether to start the component in dev or not.

    Returns:
        None
    """
    title = component_type.replace("_", " ").title()
    print(f"\nStarting {title}...")
    start: Callable[[str], None] = COMPONENT_MAP[component_type]
    start(component_type)


# MAIN
def main() -> None:
    """
    Initialises and starts the ACE / ACE component based off the starting arguments.

    Arguments:
        None

    Returns:
        None
    """
    ace_arguments: ACEArguments = assign_arguments()
    setup()
    if ace_arguments.should_update:
        ace_arguments.should_build = True
        update()
    if not ace_arguments.no_build:
        built: bool = build(ace_arguments.should_build)
        if built:
            ace_arguments.should_restart = True
    if ace_arguments.should_test:
        test()
        return
    if not ace_arguments.component == ArgumentNames.STARTUP:
        start_component(ace_arguments.component)
        return
    exists: bool = exec_check_exists(DeploymentCommands.CHECK, ACE.LOWER_NAME)
    if ace_arguments.stop:
        stop(exists)
        return
    _setup_network()
    start_ace(ace_arguments.should_restart, exists)

if __name__ == "__main__":
    main()
