"""
Layer constants for the ace_prototype.
"""

# DEPENDENCIES
## Built-In
from typing import Any, Optional
## Third-Party
from pydantic import BaseModel, validator
## Local
from constants.queue import BusKeys
from constants.layer import LayerKeys


# TYPES
class LayerSubMessage(BaseModel):
    """
    The messages of a single layer message

    Attributes:
        heading (str): The purpose of the message
        content (tuple[str, ...]): The message
    """
    heading: str
    content: tuple[str, ...]

    @validator("content", pre=True)
    def _convert_content_to_tuple(cls, content: Any) -> tuple[str, ...]:
        if isinstance(content, str):
            return (content,)
        elif isinstance(content, list):
            return tuple(content)
        elif isinstance(content, tuple):
            return content
        else:
            raise ValueError("Invalid type for content field")

class LayerMessage(BaseModel):
    """
    A single message from a layer

    Attributes:
        message_type (str): The type of message, corresponding to GUIDANCE, DATA, etc...
        messages (tuple[LayerSubMessage, ...]): The messages of that type
    """
    message_type: str
    messages: tuple[LayerSubMessage, ...]

    @validator("messages", pre=True)
    def _convert_dict_messages_to_tuple_of_sub_messages(
        cls, 
        messages: list[dict[str, str | list[str]]] | tuple[LayerSubMessage, ...]
    ) -> tuple[LayerSubMessage, ...]:
        if isinstance(messages, tuple):
            for message in messages:
                if isinstance(message, LayerSubMessage):
                    return messages
        
        if isinstance(messages, list):
            sub_messages: list[LayerSubMessage] = []
            for message in messages:
                sub_message_pre: dict = {
                    LayerKeys.HEADING: message[LayerKeys.HEADING],
                    LayerKeys.CONTENT: message[LayerKeys.CONTENT]
                }
                sub_messages.append(LayerSubMessage.model_validate(sub_message_pre))
            return tuple(sub_messages)

        return ()

LayerMessages = tuple[LayerMessage, ...]


# INTERFACE
class LayerMessageLoader:
    """
    Interface for parsing and working with layer messages.

    Attributes:
        layer_messages (LayerMessages): The layer messages.
        commands (LayerSubMessages): The commands from the messages.
        reasoning (LayerSubMessages): The reasoning from the messages.
        guidance (LayerSubMessages): The guidance from the messages.
        data (LayerSubMessages): The data from the messages.
    """
    __slots__: tuple[str, ...] = (
        "layer_messages",
        LayerKeys.COMMANDS,
        LayerKeys.INTERNAL,
        LayerKeys.GUIDANCE,
        LayerKeys.DATA
    )

    def __init__(self, raw_messages: dict) -> None:
        self.layer_messages: Optional[LayerMessages] = None
        self.commands: Optional[LayerMessage] = None
        self.internal: Optional[LayerMessage] = None
        self.guidance: Optional[LayerMessage] = None
        self.data: Optional[LayerMessage] = None
        self._load_layer_messages_from_dict(raw_messages)

    def _load_layer_messages_from_dict(self, raw_messages: dict) -> None:
        layer_messages: list[LayerMessage] = []
        for message_type, messages in raw_messages.items():
            raw_layer_message: dict = {
                LayerKeys.MESSAGE_TYPE: message_type, 
                LayerKeys.MESSAGES: messages
            }
            layer_messages.append(LayerMessage.model_validate(raw_layer_message))
        self.layer_messages = tuple(layer_messages)

        for layer_message in self.layer_messages:
            match layer_message.message_type:
                case LayerKeys.COMMANDS:
                    self.commands = layer_message
                case LayerKeys.INTERNAL:
                    self.internal = layer_message
                case BusKeys.DOWN:
                    self.guidance = layer_message
                case BusKeys.UP:
                    self.data = layer_message
