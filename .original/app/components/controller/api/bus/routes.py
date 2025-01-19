# DEPENDENCIES
## Third-Party
from fastapi import APIRouter
from starlette import responses
## Local
from constants.api import APIRoutes
from constants.queue import BusKeys
from constants.layer import LayerKeys, LayerCommands 
from components.layer.layer_messages import LayerSubMessage
from .models import BusMessage, BusResponse
from .service import bus_down, bus_up


# CONSTANTS
VONE_CHAT_ROUTE: str = f"{APIRoutes.VONE}/bus"


# API
router = APIRouter(
    prefix=VONE_CHAT_ROUTE,
    tags=["bus"],
    responses={404: {"description": "Not found"}}
)


# ROUTES
@router.post(f"/{BusKeys.DOWN}", response_model=BusResponse)
async def down(bus_message: BusMessage) -> BusResponse:
    # VERIFY PAYLOAD HERE
    print(f"Bus Message: {bus_message.model_dump_json()}")
    bus_down(bus_message)
    response = BusResponse(success=True)
    return response

@router.post(f"/{BusKeys.UP}", response_model=BusResponse)
async def up(bus_message: BusMessage) -> BusResponse:
    # VERIFY PAYLOAD HERE
    bus_up(bus_message)
    response = BusResponse(success=True)
    return response
    