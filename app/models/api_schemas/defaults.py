# DEPENDENCIES
## Third-Party
from pydantic import BaseModel


# REQUESTS


# RESPONSES
class DefaultAPIResponse(BaseModel):
    message: str
