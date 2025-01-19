# DEPENDENCIES
## Third-Party
from pydantic import BaseModel


class User(BaseModel):
    username: str

class Token(BaseModel):
    access_token: str
    token_type: str

class TokenData(BaseModel):
    username: str | None = None