# DEPENDENCIES
## Built-In
import asyncio
import json
## Local
from constants.containers import ComponentPorts
from constants.generic import GenericKeys
from constants.prompts import PromptKeys
from constants.settings import DebugLevels
from components.telemetry.api import TelemetryRequest
from helpers import debug_print, get_api
from ..layer_messages import LayerSubMessage


# STRING MANIPULATION
def build_text_from_sub_layer_messages(sub_messages: tuple[LayerSubMessage, ...], heading_identifier: str = "###") -> str:
    """
    Builds a text block from a LayerSubMessages.

    Arguments:
        sub_message (LayerSubMessage): The LayerSubMessage to build the text block from.
        heading_identifier (str): The string to use as the heading identifier (Default is ###).

    Returns:
        str: The concatenated text block generated from the messages.
    """
    debug_print(f"Building text from {sub_messages}...", DebugLevels.INFO)
    text: str = ""
    for sub_message in sub_messages:
        text += f"{heading_identifier} {sub_message.heading.title()}\n"
        if not sub_message.content:
            text += f"- {GenericKeys.NONE}"
            return text
        items: list[str] = [f"- {item}" for item in sub_message.content]
        text += "\n".join(items)
        text += "\n\n"
    return text

def build_text_from_dict(input_dict: dict[str, str]) -> str:
    output_text: str = ""
    for key, value in input_dict.items():
        output_text += f"- {key}: {value}\n"
    return output_text


# EXTERNAL SOURCES
def get_telemetry(access: frozenset[str], context_unformatted: tuple[LayerSubMessage, ...]) -> str:
    context: str = build_text_from_sub_layer_messages(sub_messages=context_unformatted)
    payload = TelemetryRequest(access=access, context=context)
    response: dict = json.loads(asyncio.run(get_api(api_port=ComponentPorts.TELEMETRY, endpoint="telemetry", payload=payload)))
    telemetry: str = build_text_from_dict(response[PromptKeys.TELEMETRY])
    return telemetry
