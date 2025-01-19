# DEPENDENCIES
## Built-In
from abc import ABC, abstractmethod
from dataclasses import dataclass
from typing import Any, Callable, final, Optional
## Local
from constants.generic import GenericKeys
from constants.settings import DebugLevels
from helpers import debug_print
from .inputs import build_text_from_sub_layer_messages
from ..layer_messages import LayerSubMessage


VariableMap = dict[str, str | frozenset[str] | tuple[LayerSubMessage, ...]]

@dataclass
class Injection(ABC):
    replace_variable_name: str

    @abstractmethod
    def get_injection(self, variable_map: VariableMap) -> str:
        raise NotImplementedError

InjectionMap = dict[str, Injection]


# VARIABLES
@final
@dataclass
class VariableInjection(Injection):
    def get_injection(self, variable_map: VariableMap) -> str:
        if not variable_map:
            raise AttributeError("No variable_map found! Assign it first using assign_variable_map before calling get_injection from a VariableInjection!")
        injection_text: Any = variable_map.get(self.replace_variable_name, GenericKeys.EMPTY)
        if isinstance(injection_text, str):
            return injection_text
        if isinstance(injection_text, tuple):
            return build_text_from_sub_layer_messages(injection_text)
        raise ValueError(f"Variable injection {self.replace_variable_name} is not a valid value type!")


# FILES
@dataclass
class BaseFileInjection(Injection):
    sub_injection_map: Optional[InjectionMap]

    @abstractmethod
    def load_file(self, variable_map: VariableMap) -> str:
        raise NotImplementedError

@final
@dataclass
class FileInjection(BaseFileInjection):
    load_file_path: str

    def load_file(self, variable_map: VariableMap) -> str:
        with open(self.load_file_path, "r", encoding="utf-8") as file:
            return file.read()

    def get_injection(self, variable_map: VariableMap) -> str:
        return self.load_file(variable_map)
    
@final
@dataclass
class ParamateriseFileInjection(BaseFileInjection):
    load_file_folder: str
    load_file_variable_name: str

    def load_file(self, variable_map: VariableMap) -> str:
        debug_print("Loading paramaterised file...", DebugLevels.INFO)
        if not variable_map:
            raise AttributeError("No variable_map found! Assign it first using assign_variable_map before calling load_file from a ParamateriseFileInjection!")
        file_name: Any = variable_map.get(self.load_file_variable_name, GenericKeys.EMPTY)
        if not isinstance(file_name, str):
            raise ValueError(f"Variable injection {self.replace_variable_name} is not a string!")
        load_file_path: str = f"{self.load_file_folder}/{file_name}"
        with open(load_file_path, "r", encoding="utf-8") as file:
            return file.read()
    
    def get_injection(self, variable_map: VariableMap) -> str:
        self.variable_map: VariableMap = variable_map
        return self.load_file(variable_map)


# METHODS
@final
@dataclass
class MethodInjectionParamaters:
    reference_variable_names: tuple[str, ...]

@final 
@dataclass
class MethodInjection(Injection):
    function: Callable
    paramaters: Optional[MethodInjectionParamaters]

    def get_injection(self, variable_map: VariableMap) -> str:
        if not variable_map:
            raise AttributeError("No variable_map found! Assign it first using assign_variable_map before calling get_injection from a MethodInjection!")

        paramaters: list[Any] = []
        if self.paramaters:
            for reference_variable_name in self.paramaters.reference_variable_names:
                paramaters.append(variable_map.get(reference_variable_name, ()))
        return self.function(*paramaters)
