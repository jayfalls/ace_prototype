# DEPENDENCIES
## Third-Party
import uvicorn
## Local
from constants.containers import ComponentPorts
from .api import api

# MAIN
def component(component_type: str) -> None:
    print(f"\nStarting {component_type} API...")
    uvicorn.run(api, host="0.0.0.0", port=int(ComponentPorts.TELEMETRY))    
