# DEPENDENCIES
## Third-Party
from fastapi import APIRouter, Request
from starlette.responses import HTMLResponse
from starlette.templating import _TemplateResponse
## Local
from constants.api import HTML_TEMPLATES


# API
router = APIRouter(
    tags=["pages"],
    responses={404: {"description": "Not found"}}
)


# PAGES
@router.get("/home", response_class=HTMLResponse)
async def home(request: Request) -> _TemplateResponse:
    context: dict = {"request": request}
    return HTML_TEMPLATES.TemplateResponse("home.html", context)

## Sub Pages
@router.get("/empty", response_class=HTMLResponse)
async def empty(request: Request) -> _TemplateResponse:
    context: dict = {"request": request}
    return HTML_TEMPLATES.TemplateResponse("empty.html", context)

@router.get("/welcome", response_class=HTMLResponse)
async def welcome(request: Request) -> _TemplateResponse:
    context: dict = {"request": request}
    return HTML_TEMPLATES.TemplateResponse("partials/welcome.html", context)

@router.get("/dashboard", response_class=HTMLResponse)
async def controls(request: Request) -> _TemplateResponse:
    context: dict = {"request": request}
    return HTML_TEMPLATES.TemplateResponse("dashboard.html", context)

@router.get("/chat", response_class=HTMLResponse)
async def chat(request: Request) -> _TemplateResponse:
    context: dict = {"request": request}
    return HTML_TEMPLATES.TemplateResponse("chat.html", context)