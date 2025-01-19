# DEPENDENCIES
## Third-Party
from fastapi import APIRouter, Request
from starlette.responses import HTMLResponse
from starlette.templating import _TemplateResponse
## Local
from constants.api import HTML_TEMPLATES, APIRoutes
from ..status.controls import start_ace


# CONSTANTS
VONE_CHAT_ROUTE: str = f"{APIRoutes.VONE}/dashboard"


# API
router = APIRouter(
    prefix=VONE_CHAT_ROUTE,
    tags=["dashboard"],
    responses={404: {"description": "Not found"}}
)


# ROUTES
@router.get("/power_on_self_test", response_class=HTMLResponse)
async def start(request: Request) -> _TemplateResponse:
    context: dict = {"request": request}
    start_ace()
    return HTML_TEMPLATES.TemplateResponse("components/dashboard/run_button.html", context)
    