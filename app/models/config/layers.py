# DEPENDENCIES
## Built-In
from typing import Literal
## Third-Party
from pydantic import BaseModel
## Local
from constants import LayerTypes, LLMModelTypes


class LayerSettings(BaseModel):
    layer_name: Literal[*LayerTypes.get_frozenset()]
    model_type: Literal[*LLMModelTypes.get_frozenset()]
