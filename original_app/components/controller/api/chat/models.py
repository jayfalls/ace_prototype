# DEPENDENCIES
## Third-Party
from pydantic import BaseModel


class ChatMessage(BaseModel):
    input_message: str
    is_ace_previous: bool
