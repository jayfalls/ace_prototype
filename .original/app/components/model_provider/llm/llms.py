# DEPENDENCIES
## Built-In
from abc import ABC, abstractmethod
from typing import Any, Generic, Iterator, Mapping, TypeVar, Union
## Third-Party
from anthropic import Anthropic
from anthropic import Stream as AnthropicStream
from anthropic.types import MessageStreamEvent
from groq import Groq
from groq.types.chat import ChatCompletion as GroqChatCompletion
import ollama
from ollama import Options as OllamaOptions
from openai import OpenAI
from openai import Stream as OpenAIStream
from openai.types.chat import ChatCompletionChunk as OpenAIChatCompletionChunk
from pydantic import BaseModel
## Local
from constants.generic import GenericKeys
from constants.model_provider import LLMKeys
from constants.settings import DebugLevels
from helpers import debug_print


# GENERIC
class LLMDetails(BaseModel):
    """Data"""
    api_key: str
    model: str
    context: int = 4096
    temperature: float = 0.2
    rate_limit: int = 20 # Seconds

GenericLLMDetails = TypeVar("GenericLLMDetails", bound=LLMDetails)
"""Hint to use a LLMDetails or a subclass of LLMDetails"""

class LLM(ABC, Generic[GenericLLMDetails]):
    """
    Abstract Base Class for Language Model Managers

    Attributes:
        api_key (str): API key for the model provider
        model (str): Model name
        context (int): Number of tokens to use for context
        temperature (float): Temperature for the model
        rate_limit (int): Minimum time between requests in seconds
    
    Methods:
        generate (system_prompt: str, assistant_begin: str) -> str: Generate a response to the system prompt
    """
    __slots__: tuple[str, ...] = (
        LLMKeys.API_KEY,
        LLMKeys.MODEL,
        LLMKeys.CONTEXT,
        LLMKeys.TEMPERATURE,
        LLMKeys.RATE_LIMIT
    )
    def __init__(self, llm_details: GenericLLMDetails) -> None:
        self.api_key: str = llm_details.api_key
        self.model: str = llm_details.model
        self.context: int = llm_details.context
        self.temperature: float = llm_details.temperature
        self.rate_limit: int = llm_details.rate_limit
        self._custom_init(llm_details)
    
    def _custom_init(self, llm_details: GenericLLMDetails) -> None:
        pass

    @abstractmethod
    def generate(self, system_prompt: str, assistant_begin: str) -> str:
        raise NotImplementedError


# CLAUDE
class ClaudeLLM(LLM):
    """
    Language Model Manager for Claude
    """
    def generate(self, system_prompt: str, assistant_begin: str) -> str:
        client = Anthropic(api_key=self.api_key)
        stream: AnthropicStream[MessageStreamEvent] = client.messages.create(
            system=system_prompt,
            messages=[
                {
                    "role": "assistant",
                    "content": assistant_begin
                }
            ],
            model=self.model,
            max_tokens=self.context,
            temperature=self.temperature,
            stream=True,
        )
        output: list[str] = [assistant_begin]
        for event in stream:
            output.append(event.type)
        return "".join(output)


# GROQ
class GroqLLM(LLM):
    """
    Language Model Manager for Groq
    """
    def generate(self, system_prompt: str, assistant_begin: str) -> str:
        client = Groq(api_key=self.api_key)
        chat_completion: GroqChatCompletion = client.chat.completions.create(
            messages=[
                {
                    "role": "system",
                    "content": system_prompt,
                }
            ],
            max_tokens=self.context,
            temperature=self.temperature,
            model=self.model,
        )
        return chat_completion.choices[0].message.content


# OLLAMA
class OllamaDetails(LLMDetails):
    """data"""
    api_key: str = GenericKeys.NONE
    low_vram: bool = False

class OllamaLLM(LLM):
    """
    Language Model Manager for Ollama

    Extra Attributes:
        low_vram (bool): Whether to run in low VRAM mode
    """
    def _custom_init(self, llm_details: OllamaDetails) -> None:
        self.__slots__ = (
            *self.__slots__,
            LLMKeys.LOW_VRAM
        )
        self.low_vram: bool = llm_details.low_vram

    def generate(self, system_prompt: str, assistant_begin: str) -> str:
        stream: Union[Mapping[str, Any], Iterator[Mapping[str, Any]]] = ollama.chat(
            model=self.model,
            messages=[
                {
                    "role": "system",
                    "content": system_prompt
                },
                {
                    "role": "assistant",
                    "content": assistant_begin
                }
            ],
            options=OllamaOptions(
                num_ctx=self.context,
                temperature=self.temperature,
                low_vram=self.low_vram
            ),
            stream=True,
        )
        output: list[str] = [assistant_begin]
        if isinstance(stream, Iterator):
            for chunk in stream:
                output.append(chunk["message"]["content"])
                debug_print(chunk["message"]["content"], DebugLevels.INFO, end="")
                print(chunk["message"]["content"], end="")
        else:
            output.append(stream["message"]["content"])
        final_output: str = "".join(output)
        print(f"Final Output: {final_output}")
        return "".join(output)


# OPENAI
class OpenAILLM(LLM):
    """
    Language Model Manager for OpenAI
    """
    def generate(self, system_prompt: str, assistant_begin: str) -> str:
        client = OpenAI(api_key=self.api_key)
        stream: OpenAIStream[OpenAIChatCompletionChunk] = client.chat.completions.create(
            model=self.model,
            messages=[
                {
                    "role": "system",
                    "content": system_prompt
                },
                {
                    "role": "assistant",
                    "content": assistant_begin
                }
            ],
            max_tokens=self.context,
            temperature=self.temperature,
            stream=True,
        )
        output: list[str] = [assistant_begin]
        for chunk in stream:
            output.append(chunk.choices[0].delta.content or "")
        return "".join(output)
