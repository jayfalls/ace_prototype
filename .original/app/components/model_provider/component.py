# DEPENEDENCIES
## Built-In
from threading import Thread
## Third-Party
import uvicorn
## local
from constants.containers import ComponentPorts
from .api import api
from .provider import startup


def component(component_type: str) -> None:
    print(f"\nStarting {component_type} API...")
    Thread(target=startup).start()
    uvicorn.run(api, host="0.0.0.0", port=int(ComponentPorts.MODEL_PROVIDER))