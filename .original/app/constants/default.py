# DEPENDENCIES
## Built-in
from abc import ABC
from typing import final, get_type_hints


# ENUMS
class BaseEnum(ABC):
    """Base Enum Class"""
    _ALLOWED_ENUM_TYPES: tuple[type, ...] = (str, int)

    def __init_subclass__(cls, **kwargs):
        super().__init_subclass__(**kwargs)
        for var_name, var_value in get_type_hints(cls).items():
            if var_name.startswith("__") or var_name.startswith("_"):
                continue
            if var_value not in cls._ALLOWED_ENUM_TYPES:
                raise TypeError(f"Attribute '{var_name}' must be of type {cls._ALLOWED_ENUM_TYPES}")
    
    @final
    @classmethod
    def get_dict(cls) -> dict[str, str]:
        base_enum_dict: dict[str, str] = {
            variable_name: value 
            for variable_name, value in vars(cls).items() 
            if not variable_name.startswith("__") and not variable_name.startswith("_")
        }
        return base_enum_dict
    
    @final
    @classmethod
    def get_values(cls) -> tuple[str, ...]:
        return tuple(cls.get_dict().values())
    
    @final
    @classmethod
    def get_frozen_values(cls) -> frozenset[str]:
        return frozenset(cls.get_values())
