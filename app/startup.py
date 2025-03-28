#!/usr/bin/env python3.13

# DEPENDENCIES
## Built-in
import argparse
import json
## Local
from constants import DictKeys, Files, Names, ShellCommands
from constants.files import setup_user_deployment_file
from logger import logger
from shell import execute_shell, shell_check_exists


# ARGUMENTS
def _get_arguments() -> dict[str, bool]:
    parser = argparse.ArgumentParser()
    parser.add_argument("-d", "--dev", action="store_true", help="Enable dev mode")
    parser.add_argument("-b", "--build", action="store_true", help="Build the container images")
    parser.add_argument("-r", "--restart", action="store_true", help="Restart the ACE cluster")
    parser.add_argument("-s", "--stop", action="store_true", help="Stop the ACE cluster")
    return vars(parser.parse_args())


# PREPARATION
def _setup_network() -> None:
    if not shell_check_exists(check_command=ShellCommands.CHECK_NETWORK, keyword=Names.NETWORK):
        logger.startup("First time setting up network...")
        execute_shell(ShellCommands.CREATE_NETWORK)

def _update() -> bool:
    """Update the git repo, returning True if container needs to be rebuilt"""
    logger.startup(f"Updating {Names.ACE}...")
    execute_shell(ShellCommands.UPDATE)

def _check_if_latest_build() -> bool:
    """Compares the update history and version files to check if the container needs to be rebuilt"""
    updates: dict[str] = {}
    history: dict[str] = {}
    with open(Files.VERSION, "r", encoding="utf-8") as updates_file:
        updates = json.load(updates_file)
    with open(Files.STARTUP_HISTORY, "r", encoding="utf-8") as history_file:
        history = json.loads(history_file.read())
    if not history:
        with open(Files.STARTUP_HISTORY, "w", encoding="utf-8") as history_file:
            history[DictKeys.REBUILD_DATE] = ""
            json.dump(history, history_file)
            return True
    
    if history.get(DictKeys.REBUILD_DATE, "") != updates[DictKeys.REBUILD_DATE]:
        return True
    return False

def _build_container_image(force_build: bool = False):
    """
    A function which builds if image doesn't exist or if force_build flag is set
    """
    logger.startup("Checking if build is required...")
    image_exists: bool = shell_check_exists(check_command=ShellCommands.CHECK_IMAGES, keyword=Names.IMAGE)
    should_build: bool = not image_exists or force_build
    if not should_build:
        logger.startup("Image already exists, skipping build...")
        return
    if not image_exists:
        logger.startup("Image does not exist, building container...")
    else:
        logger.startup("Building container...")
    execute_shell(ShellCommands.BUILD_IMAGE, error_message="Unable to build image")

    updates: dict[str] = {}
    history: dict[str] = {}
    with open(Files.VERSION, "r", encoding="utf-8") as updates_file:
        updates = json.load(updates_file)
    with open(Files.STARTUP_HISTORY, "r", encoding="utf-8") as history_file:
        history = json.load(history_file)

    history[DictKeys.REBUILD_DATE] = updates[DictKeys.REBUILD_DATE]
    with open(Files.STARTUP_HISTORY, "w", encoding="utf-8") as history_file:
        json.dump(history, history_file)


# DEPLOYMENT
def _stop_cluster() -> None:
    """
    Stops the ACE cluster if it is running
    """
    exists: bool = shell_check_exists(ShellCommands.CHECK_PODS, Names.ACE)
    if not exists:
        logger.warn(f"{Names.ACE} is not running! Cannot stop...")
        return
    logger.startup(f"Stopping {Names.ACE}...")
    execute_shell(ShellCommands.STOP_CLUSTER)

def _start_cluster(force_restart: bool) -> None:
    """
    Start the ACE cluster if it isn't running, handling restarts
    """
    exists: bool = shell_check_exists(ShellCommands.CHECK_PODS, Names.ACE)
    if not force_restart and exists:
        logger.startup(f"ACE is already running... Run with --{DictKeys.RESTART} to restart!")
        return
    if force_restart:
        logger.startup(f"Restarting {Names.ACE}...")
    else:
        logger.startup(f"Starting {Names.ACE}...")
    execute_shell(ShellCommands.DEPLOY_CLUSTER)


# MAIN
def _startup():
    arguments: dict[str, bool] = _get_arguments()
    dev: bool = arguments[DictKeys.DEV]
    force_build: bool = arguments[DictKeys.BUILD]

    setup_user_deployment_file(dev)
    _setup_network()
    if not dev:
        _update()
    update_build: bool = _check_if_latest_build()
    if update_build:
        force_build = True
    _build_container_image(force_build=force_build)
    execute_shell(ShellCommands.CLEAR_OLD_IMAGES)

    if arguments[DictKeys.STOP]:
        _stop_cluster()
        return

    _start_cluster(force_restart=arguments[DictKeys.RESTART])


if __name__ == "__main__":
    _startup()
