# DEPENDENCIES
## Third-Party
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
## Local
from .routes import root, model_provider


controller_api = FastAPI()

controller_api.include_router(root)
controller_api.include_router(model_provider)

controller_api.add_middleware(
    CORSMiddleware,
    allow_origins=["http://localhost:4200", "http://127.0.0.1:4200"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
    expose_headers=["*"]
)
