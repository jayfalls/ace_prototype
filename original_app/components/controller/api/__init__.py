# DEPENENCIES
## Third-Party
from fastapi import APIRouter
## Local
from .bus.routes import router as _bus_router
from .chat.routes import router as _chat_router
from .dashboard.routes import router as _dashboard_router
from .pages.routes import router as _pages_router
from .user.routes import router as _user_router


# CONSTANTS
ROUTERS: tuple[APIRouter, ...] = (
    _bus_router,
    _chat_router,
    _dashboard_router,
    _pages_router,
    _user_router
)