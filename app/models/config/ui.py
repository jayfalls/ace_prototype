# DEPENDENCIES
## Third-Party
from pydantic import BaseModel


class UISettings(BaseModel):
    show_footer: bool
