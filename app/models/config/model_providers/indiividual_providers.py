# DEPENDENCIES
## Third-Party
from pydantic import BaseModel


class IndividualProviderSettings(BaseModel):
    enabled: bool = False
    api_key: str = ""
