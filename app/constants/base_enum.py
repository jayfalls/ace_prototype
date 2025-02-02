# DEPENDENCIES
## Built-in
from abc import ABC
import inspect


# ENUMS
class BaseEnum(ABC):
    """Base Enum Class"""
    _ALLOWED_ENUM_TYPES: tuple[type, ...] = (str, int, float)

    def __init_subclass__(cls, **kwargs):
        super().__init_subclass__(**kwargs)
        for attr_name in dir(cls):
            if attr_name.startswith("__") or attr_name.startswith("_"):
                continue
            # Skip methods
            attr = getattr(cls, attr_name)
            if inspect.ismethod(attr) or inspect.isfunction(attr):
                continue
            if not isinstance(attr, cls._ALLOWED_ENUM_TYPES):
                raise TypeError(f"Attribute '{attr_name}' must be of type {cls._ALLOWED_ENUM_TYPES}")

    @classmethod
    def get_dict(cls) -> dict[str, str]:
        base_enum_dict: dict[str, str] = {
            variable_name: value 
            for variable_name, value in vars(cls).items() 
            if not variable_name.startswith("__") and not variable_name.startswith("_")
        }
        return base_enum_dict

    @classmethod
    def get_tuple(cls) -> tuple[str, ...]:
        return tuple(cls.get_dict().values())

    @classmethod
    def get_frozenset(cls) -> frozenset[str]:
        return frozenset(cls.get_tuple())
