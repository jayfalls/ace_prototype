# DEPENDENCIES
## Built-In
import datetime
## Third-Party
from fastapi import APIRouter, Request, status
from starlette.templating import _TemplateResponse
## Local
from constants.api import HTML_TEMPLATES, APIRoutes
from .models import ChatMessage


# CONSTANTS
VONE_CHAT_ROUTE: str = f"{APIRoutes.VONE}/chat"


# API
router = APIRouter(
    prefix=VONE_CHAT_ROUTE,
    tags=["chat"],
    responses={404: {"description": "Not found"}},
)


# ROUTES
@router.post("/listen", response_model=ChatMessage, status_code=status.HTTP_201_CREATED)
async def listen(request: Request, chat_message: ChatMessage) -> _TemplateResponse:
    message: str = chat_message.input_message
    time = datetime.datetime.now(datetime.UTC)
    context: dict = {"request": request, "message": message, "time": time.strftime("%H:%M:%S")}
    template_html_path: str = "partials/chat_bubble.html"
    if chat_message.is_ace_previous:
        user_type_context: dict = {"user_type": "me"}
        context.update(user_type_context)
        template_html_path = "partials/chat_bubble_wrapper.html"
    return HTML_TEMPLATES.TemplateResponse(template_html_path, context)