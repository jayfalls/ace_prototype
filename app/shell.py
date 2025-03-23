# DEPENDENCIES
## Built-in
import os
import subprocess
from subprocess import Popen
## Local
from logger import logger


def execute_shell(
    command: str,
    should_print_result: bool = True,
    ignore_error: bool = False,
    error_message: str = "",
    _testing: bool = False
) -> str:
    """Execute a shell command and return the output"""
    if not error_message:
        error_message = f"Unable to execute command: {command}"
    logger.debug(f'Running Command: "{command}"')

    process: Popen = subprocess.Popen(
        command,
        shell=True,  # Use shell=True to handle complex commands
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True
    )

    stdout_lines: list[str] = []
    while True:
        error_output: str = process.stderr.readline() if process.stderr else ""
        if error_output:
            stdout_lines.append(error_output)
            if should_print_result:
                print(error_output, end="")
        output: str = process.stdout.readline() if process.stdout else ""
        if output == "" and process.poll() is not None:
            break
        if output:
            stdout_lines.append(output)
            if should_print_result:
                print(output, end="")

    remaining_stdout, stderr = process.communicate()
    if remaining_stdout:
        stdout_lines.append(remaining_stdout)
        if should_print_result:
            print(remaining_stdout, end="")

    if process.returncode != 0 and not ignore_error:
        logger.error(f"{error_message}: {stderr}")
        if _testing:
            raise SystemExit(f"{error_message}: {stderr}")
        os._exit(1)

    return "".join(stdout_lines)

def shell_check_exists(shell_command: str, keyword_to_find: str) -> bool:
    """Checks if the keyword exists in the output of the check_command"""
    logger.debug(f'Checking using "{shell_command}" for {keyword_to_find}...')
    existing_terms = frozenset(execute_shell(shell_command).split("\n"))
    logger.debug(f"Existing Terms: {existing_terms}")
    for entry in existing_terms:
        if keyword_to_find in entry:
            return True
    return False
