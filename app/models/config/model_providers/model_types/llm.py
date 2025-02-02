# DEPENDENCIES
## Built-In
from typing import Literal
## Third-Party
from pydantic import BaseModel, field_validator
## Local
from constants import LLMModelTypes


class LLMModelTypeSetting(BaseModel):
    model_type: Literal[*LLMModelTypes.get_frozenset()]
    model_id: str
    logical_temperature: float
    creative_temperature: float
    output_token_limit: int
    
    @field_validator("logical_temperature")
    def validate_logical_temperature(cls, value):
        return min(max(0.0, value), 2.0)
    
    @field_validator("creative_temperature")
    def validate_creative_temperature(cls, value):
        return min(max(0.0, value), 2.0)
