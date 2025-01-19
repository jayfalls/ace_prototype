# DEPENDENCIES
## Built-in
import asyncio
from asyncio import Task, AbstractEventLoop
import os
from typing import Any
## Third-Party
from nats.aio.client import Client as NatsClient
from nats.js.client import JetStreamContext
import toml
from watchdog.observers import Observer
from watchdog.events import FileSystemEvent, FileSystemEventHandler
## Local
from components import broker
from constants.containers import VolumePaths
from constants.generic import GenericKeys, TOMLConfig
from constants.layer import LayerKeys, LayerPaths
from constants.settings import DebugLevels
from exceptions.error_handling import exit_on_error
from helpers import debug_print
from .layers import layer_factory, Layer


# CONSTANTS
BASE_CONFIG: TOMLConfig = {
    LayerKeys.BASE_INFORMATION: {
        LayerKeys.CURRENT_ACE: GenericKeys.NONE
    }
}

# SETUP
def _set_current_ace(new_ace: str) -> None:
    with open(LayerPaths.CONFIG, "r", encoding="utf-8") as config_file:
        config: TOMLConfig = toml.load(config_file)
    config[LayerKeys.BASE_INFORMATION][LayerKeys.CURRENT_ACE] = new_ace
    with open(LayerPaths.CONFIG, "w", encoding="utf-8") as config_file:
        toml.dump(config, config_file)

def _setup() -> None:
    if os.path.isfile(LayerPaths.CONFIG):
        with open(LayerPaths.CONFIG, "r", encoding="utf-8") as config_file:
            existing_config: TOMLConfig = toml.load(config_file)
            base_information: dict[str, Any] = existing_config.get(LayerKeys.BASE_INFORMATION, {})
            current_ace: str = base_information.get(LayerKeys.CURRENT_ACE, GenericKeys.NONE)
            if current_ace in existing_config.keys():
                return
            _set_current_ace(new_ace=GenericKeys.NONE)
        return
    
    with open(LayerPaths.CONFIG, "w", encoding="utf-8") as config_file:
        toml.dump(BASE_CONFIG, config_file)


# BROKER
class LayerBroker:
    def __init__(self, queue: str) -> None:
        self.nats_client: NatsClient
        self.stream: JetStreamContext
        self.queue = queue
        self.broker_task: Task
        self.running: bool = False
    
    async def run_broker(self, layer_name: str) -> None:
        print(f"Layer: {layer_name}")
        if self.running:
            await self.stop_broker()
        if not self.running and layer_name == GenericKeys.NONE:
            debug_print("Shutting broker down...", DebugLevels.INFO)
            return

        self.nats_client, self.stream = await broker.connect()
        print(f"Starting Broker for {layer_name} on {self.queue}...")
        layer: Layer = layer_factory(name=layer_name, layer_type=self.queue)
        try:
            self.broker_task: Task = asyncio.ensure_future(
                broker.subscribe(stream=self.stream, handler=layer.get_message_from_bus, queue=self.queue)
            )
            self.running = True
            await self.broker_task
        except Exception as error:
            if self.running:
                exit_on_error(f"ConnectionClosedError: {error}")
            debug_print(f"ConnectionClosedError: {error}...", DebugLevels.ERROR)
        finally:
            pass
    
    async def stop_broker(self) -> None:
        print(f"Stopping broker for {self.queue}...")
        self.running = False
        try:
            await self.nats_client.drain()
            self.broker_task.cancel()
        except Exception as error:
            pass


# RUNNING STATE
class MonitorConfig(FileSystemEventHandler):
    def __init__(self, layer_broker: LayerBroker) -> None:
        self.event_loop: AbstractEventLoop = asyncio.get_event_loop()
        self.layer_broker = layer_broker
        self.current_ace: str = GenericKeys.NONE
        self.event_loop.create_task(self.run_broker())

    def _get_config(self) -> TOMLConfig:
        with open(LayerPaths.CONFIG, "r", encoding="utf-8") as config_file:
            config: TOMLConfig = toml.load(config_file)
        return config

    def _valid_broker_change(self) -> bool:
        config: TOMLConfig = self._get_config()
        base_information: dict[str, Any] = config[LayerKeys.BASE_INFORMATION]
        config_ace: str = base_information[LayerKeys.CURRENT_ACE]
        first_key = next(iter(config))  # Get the first key in the dictionary
        del config[first_key]
        if config_ace not in config.keys() and config_ace != GenericKeys.NONE:
            debug_print(f"ACE with name {config_ace} does not exist! Create its config first before trying to instantiate its layers...", DebugLevels.ERROR)
            if self.current_ace in config.keys():
                _set_current_ace(new_ace=self.current_ace)
            else:
                _set_current_ace(new_ace=GenericKeys.NONE)
            return False
        if self.current_ace == config_ace:
            debug_print("ACE has not changed. Skipping...", DebugLevels.INFO)
            return False
        self.current_ace = config_ace
        return True

    async def run_broker(self) -> None:
        if self._valid_broker_change():
            asyncio.create_task(self.layer_broker.run_broker(layer_name=self.current_ace))

    def on_modified(self, event: FileSystemEvent) -> None:
        if event.is_directory:
            return None
        try:
            self.event_loop.create_task(self.run_broker())
        except Exception as error:
            raise error


# MAIN
async def _component(component_type: str) -> None:
    _setup()
    layer_broker = LayerBroker(queue=component_type)
    event_handler = MonitorConfig(layer_broker)
    observer = Observer()
    observer.schedule(event_handler=event_handler, path=f"{VolumePaths.HOST_LAYERS}", recursive=False)
    observer.start()
    print("Listening for config changes...")
    try:
        while True:
           await asyncio.sleep(1)
    except Exception as error:
        debug_print(f"Layer Error: {error}...", DebugLevels.ERROR)
    except KeyboardInterrupt:
        observer.stop()
    finally:
        observer.stop()
        observer.join()

def component(component_type: str) -> None:
    asyncio.run(_component(component_type))
