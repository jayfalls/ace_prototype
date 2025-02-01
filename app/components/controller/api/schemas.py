# DEPENDENCIES
## Third-Party
from pydantic import BaseModel, validator
## Local
from constants import Defaults


class SettingsSchema(BaseModel):
    # These are not required
    ace_name: str = Defaults.ACE_NAME
    model_provider: str = Defaults.MODEL_PROVIDER
    temperature: float = Defaults.TEMPERATURE

    @validator("temperature")
    def validate_temperature(cls, value):
        return min(max(0.0, value), 1.0)


# REQUESTS
EditSettingsRequest: type[BaseModel] = SettingsSchema

# RESPONSES
GetSettingsResponse: type[BaseModel] = SettingsSchema

class GetVersionDetailsResponse(BaseModel):
    version: str
