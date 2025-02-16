# DEPENDENCIES
## Third-Party
from pydantic import BaseModel


class UISettings(BaseModel):
    dark_mode: bool
    show_footer: bool
