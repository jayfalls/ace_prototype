#!/usr/bin/env python3.13

# DEPENDENCIES
## Built-in
import argparse
## Local
from constants import DictKeys, Names, ShellCommands
from constants.files import setup_user_deployment_file
from logger import logger
from shell_runner import execute_shell, exec_check_exists


# ARGUMENTS
def _get_arguments() -> dict[str, bool]:
    parser = argparse.ArgumentParser()
    parser.add_argument("-d", "--dev", action="store_true", help="Enable dev mode")
    parser.add_argument("-b", "--build", action="store_true", help="Build the container images")
    parser.add_argument("-r", "--restart", action="store_true", help="Restart the ACE cluster")
    parser.add_argument("-s", "--stop", action="store_true", help="Stop the ACE cluster")
    return vars(parser.parse_args())


# PREPARATION
def setup_network() -> None:
    if not exec_check_exists(check_command=ShellCommands.CHECK_NETWORK, keyword=Names.NETWORK):
        logger.startup("First time setting up network...")
        execute_shell(ShellCommands.CREATE_NETWORK)

def update() -> None:
    """
    Update the git repo
    """
    logger.startup(f"Updating {Names.ACE}...")
    execute_shell(ShellCommands.UPDATE)

def build_container(force_build: bool = False):
    """
    A function which builds if image doesn't exist or if force_build flag is set
    """
    logger.startup("Checking if build is required...")
    image_exists: bool = exec_check_exists(check_command=ShellCommands.CHECK_IMAGES, keyword=Names.IMAGE)
    should_build: bool = not image_exists or force_build
    if not should_build:
        logger.startup("Image already exists, skipping build...")
        return
    if not image_exists:
        logger.startup("Image does not exist, building container...")
    else:
        logger.startup("Building container...")
    execute_shell(ShellCommands.BUILD_IMAGE, error_message="Unable to build image")
    return should_build


# DEPLOYMENT
def stop_cluster() -> None:
    """
    Stops the ACE cluster if it is running
    """
    exists: bool = exec_check_exists(ShellCommands.CHECK_PODS, Names.ACE)
    if not exists:
        logger.warn(f"{Names.ACE} is not running! Cannot stop...")
        return
    logger.startup(f"Stopping {Names.ACE}...")
    execute_shell(ShellCommands.STOP_CLUSTER)

def start_cluster(force_restart: bool) -> None:
    """
    Start the ACE cluster if it isn't running, handling restarts
    """
    exists: bool = exec_check_exists(ShellCommands.CHECK_PODS, Names.ACE)
    if not force_restart and exists:
        logger.startup(f"ACE is already running... Run with --{DictKeys.RESTART} to restart!")
        return
    if force_restart:
        logger.startup(f"Restarting {Names.ACE}...")
    else:
        logger.startup(f"Starting {Names.ACE}...")
    execute_shell(ShellCommands.DEPLOY_CLUSTER)


# MAIN
def startup():
    arguments: dict[str, bool] = _get_arguments()
    dev: bool = arguments[DictKeys.DEV]

    setup_user_deployment_file(dev)
    setup_network()
    if not dev:
        update()
    build_container(force_build=arguments[DictKeys.BUILD])
    execute_shell(ShellCommands.CLEAR_OLD_IMAGES)

    if arguments[DictKeys.STOP]:
        stop_cluster()
        return

    start_cluster(force_restart=arguments[DictKeys.RESTART])


if __name__ == "__main__":
    startup()
