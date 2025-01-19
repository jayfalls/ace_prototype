# DEPENDENCIES
## Built-in
import os


# FUNCTIONS
def exit_on_error(error: str) -> None:
    print(f"ERROR: {error}")
    os._exit(1)
    