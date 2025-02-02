# DEPENDENCIES
## Built-In
from datetime import datetime
from typing import Literal
## Third-Party
from pydantic import BaseModel
## Local
from constants import ModelProviders


# BASE
class LLMModelProvider(BaseModel):
    id: str
    model_provider: Literal[*ModelProviders.get_frozenset()]
    name: str
    model_name: str
    default: bool = False
    max_input_tokens: int
    max_output_tokens: int
    cost_per_million_input_tokens: float = 0
    cost_per_million_output_tokens: float = 0
    knowledge_cutoff: datetime | None
    rate_limits: str = "Not Available"
