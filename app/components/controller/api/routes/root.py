# DEPENDENCIES
## Third-Party
from fastapi import APIRouter, HTTPException
from http import HTTPStatus
from pydantic import ValidationError
## Local
from constants import APIRoutes, Defaults, Names
from logger import logger
from models.api_schemas.controller import (
    GetVersionDetailsResponse,
    GetSettingsResponse, EditSettingsRequest
)
from models.api_schemas.defaults import DefaultAPIResponse
from ..services import root_service


root = APIRouter()

@root.get(
    f"{APIRoutes.ROOT}version",
    response_model=GetVersionDetailsResponse,
    description=f"Get the {Names.ACE}'s version data"
)
async def get_version_route() -> dict:
    try:
        return root_service.get_version()
    except ValidationError as error:
        logger.error(error)
        raise HTTPException(status_code=HTTPStatus.INTERNAL_SERVER_ERROR, detail="Version data error!")
    except Exception as error:
        logger.error(error)
        raise HTTPException(status_code=HTTPStatus.INTERNAL_SERVER_ERROR, detail=Defaults.INTERNAL_SERVER_ERROR_MESSAGE)

@root.get(
    f"{APIRoutes.ROOT}settings",
    response_model=GetSettingsResponse,
    description=f"Get the {Names.ACE} controller settings data"
)
async def get_settings_route() -> dict:
    try:
        return root_service.get_settings_data()
    except ValidationError as error:
        logger.error(error)
        raise HTTPException(status_code=HTTPStatus.INTERNAL_SERVER_ERROR, detail="Settings data error!")
    except Exception as error:
        logger.error(error)
        raise HTTPException(status_code=HTTPStatus.INTERNAL_SERVER_ERROR, detail=Defaults.INTERNAL_SERVER_ERROR_MESSAGE)

@root.post(
    f"{APIRoutes.ROOT}settings",
    response_model=DefaultAPIResponse,
    description=f"Edit the {Names.ACE} controller settings data"
)
async def set_settings_route(updated_settings: EditSettingsRequest) -> dict:
    try:
        root_service.edit_settings_data(updated_settings=updated_settings.model_dump())
        return DefaultAPIResponse(message="Settings data updated successfully!")
    except ValidationError as error:
        logger.error(error)
        raise HTTPException(status_code=HTTPStatus.INTERNAL_SERVER_ERROR, detail="Settings data error!")
    except Exception as error:
        logger.error(error)
        raise HTTPException(status_code=HTTPStatus.INTERNAL_SERVER_ERROR, detail=Defaults.INTERNAL_SERVER_ERROR_MESSAGE)

@root.delete(
    f"{APIRoutes.ROOT}settings",
    response_model=DefaultAPIResponse,
    description=f"Delete the {Names.ACE} controller settings data"
)
async def delete_settings_route() -> dict:
    try:
        root_service.delete_settings_data()
        return DefaultAPIResponse(message="Settings data deleted successfully!")
    except ValidationError as error:
        logger.error(error)
        raise HTTPException(status_code=HTTPStatus.INTERNAL_SERVER_ERROR, detail="Settings data error!")
    except Exception as error:
        logger.error(error)
        raise HTTPException(status_code=HTTPStatus.INTERNAL_SERVER_ERROR, detail=Defaults.INTERNAL_SERVER_ERROR_MESSAGE)