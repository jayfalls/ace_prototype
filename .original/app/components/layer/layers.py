"""
This module defines the Layer class and its subclasses for representing different layers in a system.

The Layer class encapsulates attributes and methods for managing layer-specific data, guidance, prompts, 
and communication with other components via a message bus. Subclasses can override certain methods
to customize behavior for specific layer types.

The module also includes helper functions for merging messages, sending requests to other components,
and instantiating Layer objects using a factory function.
"""

# DEPENDENCIES
## Built-In
import asyncio
from threading import Thread
from typing import Any, final, Optional, Union
## Third-Party
from nats.aio.msg import Msg as NatsMsg
from pydantic import ValidationError
import toml
## Local
from constants.containers import ComponentPorts
from constants.generic import GenericKeys
from constants.layer import (
    LayerKeys, Layers, LayerCommands, LayerPaths
)
from constants.model_provider import LLMStackTypes
from constants.prompts import PromptFilePaths, PromptKeys
from constants.queue import BusKeys
from constants.settings import DebugLevels
from components.controller.api.bus.models import BusMessage, BusResponse
from components.model_provider import ModelPrompt, ModelResponse
from helpers import debug_print, get_api, post_api
from .injections import InjectionMap, BASE_PROMPT_MAP, ASPIRATIONAL_PROMPT_MAP, OUTPUT_RESPONSE_MAP
from .layer_messages import LayerMessage, LayerMessageLoader, LayerSubMessage
from .presets import LayerPreset, LAYER_PRESET_MAP
from .prompt_builder import build_prompt, VariableMap


# CONSTANTS
_ASSISTANT_BEGIN: str = '```toml\n[internal]\nreasoning = """'
"""Must match the start of the response schemas"""


# PARSING
def _merge_messages(new_messages: tuple[LayerSubMessage, ...], old_messages: tuple[LayerSubMessage, ...]) -> tuple[LayerSubMessage, ...]:
    debug_print("Merging Messages...", DebugLevels.INFO)
    merged_messages: list[LayerSubMessage] = []
    old_headings: tuple[str, ...] = tuple([old_message.heading for old_message in old_messages])
    matched_headings: list[str] = []
    for new_message in new_messages:
        if new_message.heading in old_headings:
            matched_headings.append(new_message.heading)
            index: int = old_headings.index(new_message.heading)
            original_content: tuple[str, ...] = old_messages[index].content
            merged_messages.append(LayerSubMessage(
                heading=new_message.heading,
                content=(*original_content, *new_message.content),
            ))
            continue
        merged_messages.append(new_message)
    for index, old_heading in enumerate(old_headings):
        if old_heading in matched_headings:
            continue
        merged_messages.append(old_messages[index])
    return tuple(merged_messages)


# COMMUNICATION
async def _try_send(direction: str, source_queue: str, layer_message: LayerMessage) -> BusResponse:
    bus_message = BusMessage(source_queue=source_queue, layer_message=layer_message)
    response: str = await post_api(api_port=ComponentPorts.CONTROLLER, endpoint=f"bus/{direction}", payload=bus_message)
    response_validated = BusResponse.model_validate_json(response)
    return response_validated

async def _model_response(system_prompt: str) -> ModelResponse:
    model_request = ModelPrompt(stack_type=LLMStackTypes.GENERALIST, system_prompt=system_prompt, assistant_begin=_ASSISTANT_BEGIN)
    response: str = await get_api(api_port=ComponentPorts.MODEL_PROVIDER, endpoint="generate", payload=model_request)
    response_validated = ModelResponse.model_validate_json(response)
    return response_validated


# BASE LAYER
BASE_PROMPT_MAPS: dict[str, InjectionMap] = {
    Layers.ASPIRATIONAL: ASPIRATIONAL_PROMPT_MAP,
    Layers.GLOBAL_STRATEGY: BASE_PROMPT_MAP,
    Layers.AGENT_MODEL: BASE_PROMPT_MAP,
    Layers.EXECUTIVE_FUNCTION: BASE_PROMPT_MAP,
    Layers.COGNITIVE_CONTROL: BASE_PROMPT_MAP,
    Layers.TASK_PROSECUTION: BASE_PROMPT_MAP
}

class Layer:
    """
    Attributes:
        name (str): The name of the layer
        layer_type (str): The type of the layer
        base_prompt (str): The base system prompt for this layer
        guidance (tuple[LayerSubMessage]): The guidance for this layer
        data (tuple[LayerSubMessage]): The data for this layer
        telemetry (frozenset[str]): The telemetry inputs this layer has access to
        first_run (bool): Whether this layer has been run for the first time
        processing (bool): Whether this layer is currently processing
        max_retries (int): The maximum number of model_provider retries this layer can do
        default_guidance (tuple[LayerSubMessage]): The default guidance for this layer
        has_data (bool): Whether this layer has data
        default_data (tuple[LayerSubMessage]): The default data for this layer
    """
    __slots__: list[str] = [
        LayerKeys.NAME,
        LayerKeys.TYPE,
        LayerKeys.BASE_PROMPT,
        LayerKeys.GUIDANCE,
        LayerKeys.DATA,
        LayerKeys.TELEMETRY,
        LayerKeys.FIRST_RUN,
        LayerKeys.PROCESSING,
        LayerKeys.MAX_RETRIES,
        LayerKeys.DEFAULT_GUIDANCE,
        LayerKeys.HAS_DATA,
        LayerKeys.DEFAULT_DATA
    ]
    @final
    def __init__(self, name: str, layer_type: str, preset: LayerPreset) -> None:
        self.name: str = name
        self.layer_type: str = layer_type
        self.guidance: tuple[LayerSubMessage, ...] = ()
        self.data: tuple[LayerSubMessage, ...] = ()
        self.telemetry: frozenset[str] = preset.TELEMETRY
        self.first_run: bool = True
        self.processing: bool = False
        self.max_retries: int = 3
        self.default_guidance: tuple[LayerSubMessage, ...] = ()
        self.has_data: bool = True
        self.default_data: tuple[LayerSubMessage, ...] = ()
        self._custom_init()
        self.base_prompt: str = ""
        self.base_prompt = self._build_base_prompt()
    
    def _custom_init(self) -> None:
        pass
    
    # Layer Controls
    @final
    async def _process_controller_message(self, layer_message: LayerMessage) -> None:
        actions: tuple[str, ...] = tuple([sub_message.heading for sub_message in layer_message.messages])
        for action in actions:
            match action:
                case LayerCommands.POST:
                    await _try_send(
                        direction=BusKeys.DOWN,
                        source_queue=self.layer_type,
                        layer_message=layer_message,
                    )
                case _:
                    print(f"{action} Does not match any known layer actions!")

    # System Prompt
    @final
    def _get_variable_map(self) -> VariableMap:
        return {slot: getattr(self, slot) for slot in self.__slots__}

    @final
    def _build_base_prompt(self) -> str:
        with open(PromptFilePaths.LAYER, "r", encoding="utf-8") as base_prompt_file:
            base_prompt_text: str = base_prompt_file.read()
        base_prompt: str = build_prompt(
            text_with_variables=base_prompt_text,
            injection_map=BASE_PROMPT_MAPS[self.layer_type],
            variable_map=self._get_variable_map()
        )
        debug_print(f"{self.layer_type} Base Prompt: {base_prompt}", DebugLevels.INFO)
        return base_prompt

    # Layer Messages
    @final
    def _process_layer_message(self) -> None:
        print(f"Checking if should process {self.layer_type}...")
        if self.processing:
            print(f"{self.layer_type} still processing...")
            return
        has_enough_data: bool = self.guidance != self.default_guidance and self.data != self.default_data
        if not self.has_data:
            has_enough_data = self.guidance != self.default_guidance
        if has_enough_data or self.first_run:
            self.first_run = False
            print(f"Processing {self.layer_type}...")
            self.processing = True
            output_response_prompt: str = build_prompt(
                text_with_variables=self.base_prompt,
                injection_map=OUTPUT_RESPONSE_MAP,
                variable_map={**self._get_variable_map(), PromptKeys.CONTEXT: (*self.guidance, *self.data)}
            )
            self.guidance = self.default_guidance
            self.data = self.default_data
            debug_print(f"Output Response Prompt for {self.layer_type}: {output_response_prompt}", DebugLevels.INFO)
            is_output_valid: bool = False
            formatted_response: dict[str, dict[str, Union[str, list[str]]]] = {}
            print("Getting output from model...")
            retries: int = 0
            while not is_output_valid and retries < self.max_retries:
                response: ModelResponse = asyncio.run(_model_response(system_prompt=output_response_prompt))
                response_text: str = response.response
                if response_text.startswith("```toml"):
                    response_text = response_text.replace("```toml", "")
                try:
                    formatted_response = toml.loads(response_text)
                    is_output_valid = True
                except Exception as error:
                    debug_print(f"Error formatting llm response from {self.layer_type}: {error}", DebugLevels.WARNING)
                finally:
                    retries += 1
                print(f"Retries: {retries}")
            if not is_output_valid:
                print(f"Failed to get output from {self.layer_type} after {self.max_retries} retries!")
                self.processing = False
                return
            print("Received Output...")
            print(f"\nModel Response:\n{formatted_response}\n", DebugLevels.INFO)
            # LOG internal.reasoning HERE
            layer_message_loader = LayerMessageLoader(formatted_response)
            southbound: Optional[LayerMessage] = layer_message_loader.guidance
            if southbound:
                asyncio.run(_try_send(
                    direction=BusKeys.DOWN,
                    source_queue=self.layer_type,
                    layer_message=southbound
                ))
            northbound: Optional[LayerMessage] = layer_message_loader.data
            if northbound:
                asyncio.run(_try_send(
                    direction=BusKeys.UP,
                    source_queue=self.layer_type,
                    layer_message=northbound
                ))
            self.processing = False
            return
        print(f"{self.layer_type} waiting to have enough guidance and data to process...")

    # Queue Processing
    @final
    async def get_message_from_bus(self, message: NatsMsg) -> None:
        try:
            layer_message = LayerMessage.model_validate_json(message.data)
        except ValidationError as error:
            print(f"Incorrect message format!\n{error}")
            return
        print(f"Received {layer_message.model_dump_json()} on {message.subject} queue...")
        if layer_message.message_type == LayerKeys.COMMANDS:
            asyncio.create_task(self._process_controller_message(layer_message))
        else:
            match layer_message.message_type:
                case LayerKeys.GUIDANCE:
                    self.guidance = _merge_messages(new_messages=layer_message.messages, old_messages=self.guidance)
                case LayerKeys.DATA:
                    self.data = _merge_messages(new_messages=layer_message.messages, old_messages=self.data)
            thread = Thread(target=self._process_layer_message)
            thread.start()
        await message.ack()


# INHERITED
class Aspirational(Layer):
    def _custom_init(self) -> None:
        self.default_guidance = (
            LayerSubMessage(
                heading=GenericKeys.NONE, 
                content=(GenericKeys.EMPTY,)
            ),
        )
        self.__slots__.append(LayerKeys.MISSION)
        with open(LayerPaths.CONFIG, "r", encoding="utf-8") as config_file:
            config = toml.load(config_file)
        base_information: dict[str, Any] = config.get(self.name, {})
        self.ace_mission: str = base_information.get(LayerKeys.MISSION, "")

class TaskProsecution(Layer):
    def _custom_init(self) -> None:
        self.has_data = False


# INSTANTIATION
LAYER_MAP: dict[str, type[Layer]] = {
    Layers.ASPIRATIONAL: Aspirational,
    Layers.GLOBAL_STRATEGY: Layer,
    Layers.AGENT_MODEL: Layer,
    Layers.EXECUTIVE_FUNCTION: Layer,
    Layers.COGNITIVE_CONTROL: Layer,
    Layers.TASK_PROSECUTION: TaskProsecution
}

def layer_factory(name: str, layer_type: str) -> Layer:
    try:
        layer: type[Layer] = LAYER_MAP[layer_type]
        preset: type[LayerPreset] = LAYER_PRESET_MAP[layer_type]
        return layer(name=name, layer_type=layer_type, preset=preset())
    except Exception as error:
        raise error
