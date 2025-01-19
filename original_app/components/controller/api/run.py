# DEPENDENCIES
## Built-In
from typing import Optional
## Third-Party
from fastapi import FastAPI, Request, Header
from fastapi.responses import FileResponse
from fastapi.staticfiles import StaticFiles
from starlette.responses import HTMLResponse
from starlette.templating import _TemplateResponse
## Local
from . import ROUTERS
from constants.api import APIPaths, HTML_TEMPLATES
from .user.service import test_toml


# SETUP
app = FastAPI()
_ = [app.include_router(router) for router in ROUTERS]
app.mount("/assets", StaticFiles(directory=f"{APIPaths.UI}/assets"), name="assets")


# ROUTES
@app.get("/favicon.ico", include_in_schema=False)
async def favicon() -> FileResponse:
    return FileResponse(APIPaths.FAVICON)

@app.get("/", response_class=HTMLResponse)
async def root(request: Request) -> _TemplateResponse:
    context: dict = {"request": request}
    #test_toml()
    return HTML_TEMPLATES.TemplateResponse("root.html", context)


# TESTING
## Constants
example_table: tuple[dict[str, str], ...] = (
    {
        "test1": "Polompon",
        "test2": "Radical",
        "test3": "Psoso"
    },
    {
        "test1": "Trumpin",
        "test2": "Restified",
        "test3": "Adalac"
    }
)

## Routes
@app.get("/test", response_class=HTMLResponse)
async def test(request: Request, hx_request: Optional[str] = Header(default=None)) -> _TemplateResponse:
    context: dict = {"request": request, "name": "ACE Controller Dashboard", "tests": example_table}
    if hx_request:
        return HTML_TEMPLATES.TemplateResponse("partials/table.html", context)
    return HTML_TEMPLATES.TemplateResponse("test.html", context)

@app.get("/test2", response_class=HTMLResponse)
async def test2(request: Request) -> _TemplateResponse:
    context: dict = {"request": request, "name": "ACE Controller Dashboard", "tests": example_table}
    return HTML_TEMPLATES.TemplateResponse("test.html", context)
