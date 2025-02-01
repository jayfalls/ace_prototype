# DEPENDENCIES
## Built-in
import os
## Local
from constants import ShellCommands
from logger import logger
from shell_runner import execute_shell

def start_ui(component_type: str, dev: bool) -> None:
  logger.startup(f"Starting {component_type}...")
  module_path: str = os.path.dirname(os.path.abspath(__file__))
  os.chdir(module_path)
  execute_shell(ShellCommands.INSTALL_NPM_DEPENDENCIES)
  if dev:
    execute_shell(ShellCommands.RUN_UI_DEV)
