# DEPENDENCIES
## Third-Party
from fastapi import FastAPI
from pydantic import BaseModel
## Local
from helpers import debug_print
from constants.api import APIRoutes
from constants.settings import DebugLevels
from .scraper import collect_telemetry


# VALIDATION
class TelemetryRequest(BaseModel):
    access: frozenset[str]
    context: str

class TelemetryResponse(BaseModel):
    telemetry: dict[str, str]

class UserInputRequest(BaseModel):
    user_input: str

class UserInputResponse(BaseModel):
    success: bool
    message: str


# SETUP
api = FastAPI()


# ROUTES
@api.get(f"{APIRoutes.VONE}/telemetry", response_model=TelemetryResponse)
async def get_telemetry(request: TelemetryRequest) -> TelemetryResponse:
    print("Generating telemetry...")
    telemetry: dict[str, str] = collect_telemetry(access=request.access, context=request.context)
    debug_print(f"Telemetry: {telemetry}", debug_level=DebugLevels.INFO)
    model_response = TelemetryResponse(telemetry=telemetry)
    return model_response

@api.post(f"{APIRoutes.VONE}/user_input", response_model=UserInputResponse)
async def post_user_input(request: UserInputRequest) -> UserInputResponse:
    debug_print(f"User Input: {request.user_input}", debug_level=DebugLevels.INFO)
    model_response = UserInputResponse(success=True, message="Success")
    return model_response
