# DEPENDENCIES
## Third-Party
from fastapi import APIRouter, HTTPException
from http import HTTPStatus
from pydantic import ValidationError
## Local
from constants import APIRoutes, Defaults, Names
from logger import logger
from models.api_schemas.controller import GetLLMModelsResponse
from ..services import model_provider_service


model_provider = APIRouter()

@model_provider.get(
    f"{APIRoutes.MODEL_PROVIDER}llm/model-types",
    response_model=tuple[str, ...],
    description=f"Get the {Names.ACE} available LLM model types"
)
async def get_llm_model_types_route() -> tuple[str, ...]:
    try:
        return model_provider_service.get_llm_model_types()
    except ValidationError as error:
        logger.error(error)
        raise HTTPException(status_code=HTTPStatus.INTERNAL_SERVER_ERROR, detail="LLM model types data error!")
    except Exception as error:
        logger.error(error)
        raise HTTPException(status_code=HTTPStatus.INTERNAL_SERVER_ERROR, detail=Defaults.INTERNAL_SERVER_ERROR_MESSAGE)

@model_provider.get(
    f"{APIRoutes.MODEL_PROVIDER}llm/models",
    response_model=list[GetLLMModelsResponse],
    description=f"Get the {Names.ACE} available LLM models"
)
async def get_llm_models_route() -> list[GetLLMModelsResponse]:
    try:
        return model_provider_service.get_llm_models()
    except ValidationError as error:
        logger.error(error)
        raise HTTPException(status_code=HTTPStatus.INTERNAL_SERVER_ERROR, detail="LLM model types data error!")
    except Exception as error:
        logger.error(error)
        raise HTTPException(status_code=HTTPStatus.INTERNAL_SERVER_ERROR, detail=Defaults.INTERNAL_SERVER_ERROR_MESSAGE)
