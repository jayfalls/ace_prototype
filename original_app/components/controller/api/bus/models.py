# DEPENDENCIES
## Third-Party
from typing import Literal
from pydantic import BaseModel
## Local
from components.layer.layer_messages import LayerMessage


# REQUESTS
class BusMessage(BaseModel):
    source_queue: str
    layer_message: LayerMessage


# RESPONSES
class BusResponse(BaseModel):
    success: bool
