# DEPENDENCIES
## Third-Party
from pydantic import BaseModel
## Local
from models.config.controller import ControllerSettingsSchema
from models.data.model_providers import LLMModelProvider


# REQUESTS
EditSettingsRequest: type[BaseModel] = ControllerSettingsSchema


# RESPONSES
class GetVersionDetailsResponse(BaseModel):
    version: str

GetSettingsResponse: type[BaseModel] = ControllerSettingsSchema

GetLLMModelsResponse: type[BaseModel] = LLMModelProvider
