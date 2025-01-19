# DEPENDENCIES
## Built-in
import subprocess
from subprocess import Popen
import sys
from time import time
from typing import Any, IO, Optional
## Third-Party
import aiohttp
import httpx
from pydantic import BaseModel
## Local
from config import Settings
from constants.api import APIRoutes
from constants.containers import ComponentPorts
from constants.settings import DebugLevels
from exceptions.error_handling import exit_on_error


# LOGGING
def debug_print(message: str, debug_level: int = DebugLevels.INFO, end: Optional[str] = None) -> None:
    if not Settings.DEBUG_LEVEL >= debug_level:
        return
    if end:
        print(message, end=end)
    else:    
        print(message)


# SHELL
def execute(
    command: str,
    should_print_result: bool = True,
    ignore_error: bool = False,
    error_message: str = "",
    debug_level: int = DebugLevels.ERROR
) -> str:
    """
    Execute a shell command and return the output

    Arguments:
        command (str): The shell command to execute
        should_print_result (bool): Whether to print the result
        ignore_error (bool): Whether to ignore any errors
        error_message (str): The error message to display if ignore_error is False
        debug_level (int): The debug level to use for logging

    Returns:
        str: The output of the shell command
    """
    if not error_message:
        error_message = f"Unable to execute command: {command}"
    debug_print(f"Running Command: {command}", debug_level=4)
    command_list: tuple[str, ...] = tuple(command.split())
    process: Popen = subprocess.Popen(command_list, stdout=subprocess.PIPE, text=True)
    if should_print_result:
        has_printed: bool = False
        while process.poll() is None:
            if not process.stdout:
                continue
            print_lines: IO = process.stdout
            if has_printed:
                for _ in print_lines:
                    sys.stdout.write("\033[F")  # Move cursor up one line
                    sys.stdout.write("\033[K") # Clear line
            for line in print_lines:
                debug_print(line, debug_level=debug_level, end="")
            has_printed = True
    if process.returncode != 0 and not ignore_error:
        exit_on_error(f"{error_message}\n{process.stderr}")
    stdout, stderr = process.communicate()
    return stdout

def exec_check_exists(check_command: str, keyword: str) -> bool:
    """
    Checks if the keyword exists in the output of the check_command

    Arguments:
        check_command (str): The shell command to used to check if the keyword exists
        keyword (str): The keyword to check for
    
    Returns:
        bool: True if the keyword exists, False otherwise
    """
    debug_print(f"\nChecking using {check_command} for {keyword}...", DebugLevels.DEBUG)
    existing: frozenset = frozenset(execute(check_command, debug_level=DebugLevels.DEBUG).split("\n"))
    debug_print(f"Existing Terms: {existing}", DebugLevels.DEBUG)
    for entry in existing:
        if keyword in entry:
            return True
    return False


# API REQUESTS
async def get_api(api_port: str, endpoint: str, payload: BaseModel) -> str:
    """
    Sends a GET request to the specified API endpoint with the provided payload and returns the response as a string

    Arguments:
        api_port (str): The API port to send the request to
        endpoint (str): The API endpoint to send the request to
        payload (BaseModel): The payload to send in the request
    
    Returns:
        str: The response json as a string
    """
    if api_port not in ComponentPorts.get_frozen_values():
        raise ValueError(f"Invalid API Port: {api_port}")
    print(f"Send Payload: {payload.model_dump_json()}")
    async with aiohttp.ClientSession() as session:
        async with session.get(
            url=f"http://127.0.0.1:{api_port}{APIRoutes.VONE}/{endpoint}", 
            data=payload.model_dump_json(), 
            headers={'Content-Type': 'application/json'},
            timeout=900
        ) as response:
            print("Api Response Status:", response.status)
            print("Api Response Content-type:", response.headers['content-type'])
            html: str = await response.text()
            print("Api Response Body:", html)
            return html

async def post_api(api_port: str, endpoint: str, payload: BaseModel) -> str:
    """
    Sends a POST request to the specified API endpoint with the provided payload and returns the response as a string

    Arguments:
        api_port (str): The API port to send the request to
        endpoint (str): The API endpoint to send the request to
        payload (BaseModel): The payload to send in the request
    
    Returns:
        str: The response json as a string
    """
    if api_port not in ComponentPorts.get_frozen_values():
        raise ValueError(f"Invalid API Port: {api_port}")
    print(f"Send Payload: {payload.model_dump_json()}")
    async with aiohttp.ClientSession() as session:
        async with session.post(
            url=f"http://127.0.0.1:{api_port}{APIRoutes.VONE}/{endpoint}", 
            data=payload.model_dump_json(), 
            headers={'Content-Type': 'application/json'},
            timeout=900
        ) as response:
            print("Status:", response.status)
            print("Content-type:", response.headers['content-type'])
            html: str = await response.text()
            print("Body:", html, "...")
            return html

def check_internet_access() -> bool:
    """
    Check if the device has internet access
    
    Returns:
        bool: True if the device has internet access, False otherwise
    """
    try:
        response = httpx.get("https://www.google.com/")
        response.raise_for_status()
        return True
    except httpx.RequestError:
        return False


# CACHE
class KeyValueCacheStore:
    """
    A simple key-value cache store

    Attributes:
        store (dict): The cache store
        ttl_map (dict): The time-to-live (TTL) map for each key
        fetches (int): The number of total fetches
        individual_fetches (dict): The number of individual fetches for each key
    
    Methods:
        get(self, key: str) -> Optional[Any]
        set(self, key: str, value: Any, ttl_seconds: Optional[int] = None) -> None
        invalidate(self, key: str) -> None
        clear(self) -> None
        get_stats(self) -> dict
    """
    def __init__(self):
        self.store: dict[str, Any] = {}
        self.ttl_map: dict[str, dict[str, int]] = {}
        self.fetches: int = 0
        self.individual_fetches: dict[str, int] = {}

    def get(self, key: str) -> Optional[Any]:
        """
        Get a value from the cache

        Arguments:
            key (str): The key to get the value for
        
        Returns:
            Any: The value from the cache, or None if the key is not found
        """
        self.fetches += 1
        self.individual_fetches[key] = self.individual_fetches.get(key, 0) + 1
        if key in self.ttl_map:
            if int(time()) > self.ttl_map[key]["expiry_time"]:
                self.invalidate(key)
                return None
            return self.store[key]
        return self.store.get(key, None)

    def set(self, key: str, value: Any, ttl_seconds: Optional[int] = None) -> None:
        """
        Set a value in the cache. If ttl_seconds is provided, the value will expire after that many seconds

        Arguments:
            key (str): The key to store the value under
            value (Any): The value to store
            ttl_seconds (Optional[int]): The number of seconds until the value expires
        """
        self.store[key] = value
        if ttl_seconds:
            ttl_map: dict[str, int] = {
                "ttl": ttl_seconds,
                "expiry_time": int(time()) + ttl_seconds
            }
            self.ttl_map[key] = ttl_map

    def invalidate(self, key):
        """
        Invalidate a value in the cache

        Arguments:
            key (str): The key to invalidate
        
        Raises:
            KeyError: If the key is not found
        """
        try:
            del self.store[key]
        except KeyError:
            raise KeyError(f"Key not found: {key}")

    def clear(self):
        """
        Clear the entire cache
        """
        self.store.clear()
        self.ttl_map.clear()
        self.fetches = 0
        self.individual_fetches.clear()

    def get_stats(self):
        """
        Get the statistics for the cache

        Returns:
            dict: The statistics for the cache
        """
        return {"size": len(self.store), "fetches": self.fetches, "individual_fetches": self.individual_fetches}
