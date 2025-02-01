# DEPENDENCIES
## Third-Party
from fastapi import FastAPI, HTTPException
from fastapi.middleware.cors import CORSMiddleware
from http import HTTPStatus
from pydantic import ValidationError
## Local
from constants import Defaults, DefaultAPIResponseSchema, Names
from logger import logger
from . import service
from .schemas import (
    EditSettingsRequest,
    GetSettingsResponse, GetVersionDetailsResponse
)


controller_api = FastAPI()
controller_api.add_middleware(
    CORSMiddleware,
    allow_origins=["http://localhost:4200"],  # Allow requests from your Angular app
    allow_credentials=True,
    allow_methods=["*"],  # Allow all HTTP methods
    allow_headers=["*"],  # Allow all headers
)


# ROUTES
@controller_api.get(
    "/settings",
    response_model=GetSettingsResponse,
    description=f"Get the {Names.ACE} controller settings data"
)
async def get_settings_route() -> dict:
    try:
        return service.get_settings_data()
    except ValidationError as error:
        logger.error(error)
        raise HTTPException(status_code=HTTPStatus.INTERNAL_SERVER_ERROR, detail="Settings data error!")
    except Exception as error:
        logger.error(error)
        raise HTTPException(status_code=HTTPStatus.INTERNAL_SERVER_ERROR, detail=Defaults.INTERNAL_SERVER_ERROR_MESSAGE)

@controller_api.post(
    "/settings",
    response_model=DefaultAPIResponseSchema,
    description=f"Edit the {Names.ACE} controller settings data"
)
async def set_settings_route(updated_settings: EditSettingsRequest) -> dict:
    try:
        service.edit_settings_data(updated_settings=updated_settings.dict())
        return DefaultAPIResponseSchema(message="Settings data updated successfully!")
    except ValidationError as error:
        logger.error(error)
        raise HTTPException(status_code=HTTPStatus.INTERNAL_SERVER_ERROR, detail="Settings data error!")
    except Exception as error:
        logger.error(error)
        raise HTTPException(status_code=HTTPStatus.INTERNAL_SERVER_ERROR, detail=Defaults.INTERNAL_SERVER_ERROR_MESSAGE)

@controller_api.get(
    "/version",
    response_model=GetVersionDetailsResponse,
    description=f"Get the {Names.ACE}'s version data"
)
async def get_version_route() -> dict:
    try:
        return service.get_version_data()
    except ValidationError as error:
        logger.error(error)
        raise HTTPException(status_code=HTTPStatus.INTERNAL_SERVER_ERROR, detail="Version data error!")
    except Exception as error:
        logger.error(error)
        raise HTTPException(status_code=HTTPStatus.INTERNAL_SERVER_ERROR, detail=Defaults.INTERNAL_SERVER_ERROR_MESSAGE)
