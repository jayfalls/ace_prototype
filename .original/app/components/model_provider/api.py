# DEPENDENCIES
## Third-Party
from fastapi import FastAPI
from pydantic import BaseModel
## Local
from helpers import debug_print
from constants.api import APIRoutes
from constants.settings import DebugLevels
from .provider import generate_response


# VALIDATION
class ModelPrompt(BaseModel):
    stack_type: str
    system_prompt: str
    assistant_begin: str = " "

class ModelResponse(BaseModel):
    response: str


# SETUP
api = FastAPI()


# ROUTES
@api.get(f"{APIRoutes.VONE}/generate", response_model=ModelResponse)
async def generate(prompt: ModelPrompt) -> ModelResponse:
    print(f"Generating response for {prompt.stack_type}...")
    response: str = generate_response(stack_type=prompt.stack_type, system_prompt=prompt.system_prompt, assistant_begin=prompt.assistant_begin)
    debug_print(f"Response: {response}", debug_level=DebugLevels.INFO)
    model_response = ModelResponse(response=response)
    return model_response
