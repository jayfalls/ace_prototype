# DEPENDENCIES
## Third-Party
from pydantic import BaseModel


class IndividualProviderSettings(BaseModel):
    name: str
    enabled: bool = False
    api_key: str = ""
