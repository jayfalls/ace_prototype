"""
Argument constants for the ace_prototype.
"""


# DEPENDENCIES
## Local
from .components import ComponentTypes
from .default import BaseEnum


class ArgumentNames(BaseEnum):
    """Enum"""
    DEV: str = "dev"
    TEST: str = "test"
    BUILD: str = "build"
    SKIP_BUILD: str = "skip_build"
    STARTUP: str = "startup"
    STOP: str = "stop"
    RESTART: str = "restart"
    UPDATE: str = "update"
    COMPONENT_TYPE: str = "component_type"

ARGUMENTS_SHORT: dict[str, str] = {
    ArgumentNames.DEV: "-d",
    ArgumentNames.TEST: "-t",
    ArgumentNames.BUILD: "-b",
    ArgumentNames.SKIP_BUILD: "-sb",
    ArgumentNames.STARTUP: "-s",
    ArgumentNames.STOP: "-s",
    ArgumentNames.RESTART: "-r",
    ArgumentNames.UPDATE: "-u",
    ArgumentNames.COMPONENT_TYPE: "-ct"
}

ARGUMENTS_LONG: dict[str, str] = {
    ArgumentNames.DEV: "--dev",
    ArgumentNames.TEST: "--test",
    ArgumentNames.BUILD: "--build",
    ArgumentNames.SKIP_BUILD: "--skip-build",
    ArgumentNames.STARTUP: "--startup",
    ArgumentNames.STOP: "--stop",
    ArgumentNames.RESTART: "--restart",
    ArgumentNames.UPDATE: "--update",
    ArgumentNames.COMPONENT_TYPE: "--component-type"
}

ARGUMENTS: dict[str, frozenset] = {
    ArgumentNames.DEV: frozenset((ARGUMENTS_SHORT[ArgumentNames.DEV], ARGUMENTS_LONG[ArgumentNames.DEV])),
    ArgumentNames.TEST: frozenset((ARGUMENTS_SHORT[ArgumentNames.TEST], ARGUMENTS_LONG[ArgumentNames.TEST])),
    ArgumentNames.BUILD: frozenset((ARGUMENTS_SHORT[ArgumentNames.BUILD], ARGUMENTS_LONG[ArgumentNames.BUILD])),
    ArgumentNames.SKIP_BUILD: frozenset((ARGUMENTS_SHORT[ArgumentNames.SKIP_BUILD], ARGUMENTS_LONG[ArgumentNames.SKIP_BUILD])),
    ArgumentNames.STOP: frozenset((ARGUMENTS_SHORT[ArgumentNames.STOP], ARGUMENTS_LONG[ArgumentNames.STOP])),
    ArgumentNames.RESTART: frozenset((ARGUMENTS_SHORT[ArgumentNames.RESTART], ARGUMENTS_LONG[ArgumentNames.RESTART])),
    ArgumentNames.UPDATE: frozenset((ARGUMENTS_SHORT[ArgumentNames.UPDATE], ARGUMENTS_LONG[ArgumentNames.UPDATE])),
    ArgumentNames.COMPONENT_TYPE: frozenset((ARGUMENTS_SHORT[ArgumentNames.COMPONENT_TYPE], ARGUMENTS_LONG[ArgumentNames.COMPONENT_TYPE])),
}

BOOL_ARGUMENTS: tuple[str, ...] = (
    ArgumentNames.DEV,
    ArgumentNames.TEST,
    ArgumentNames.BUILD,
    ArgumentNames.SKIP_BUILD,
    ArgumentNames.STOP,
    ArgumentNames.RESTART,
    ArgumentNames.UPDATE
)

STRING_ARGUMENTS: tuple[str, ...] = (
    ArgumentNames.COMPONENT_TYPE,
)


ARGUMENTS_HELP: dict[str, str] = {
    ArgumentNames.DEV: "Enable debug mode",
    ArgumentNames.TEST: "Run tests",
    ArgumentNames.BUILD: "Build the images",
    ArgumentNames.SKIP_BUILD: "Skip the build",
    ArgumentNames.STOP: "Stop the ACE cluster",
    ArgumentNames.RESTART: "Restart the ACE cluster",
    ArgumentNames.UPDATE: "Update the ACE",
    ArgumentNames.COMPONENT_TYPE: f"Select the component type. Available types: {', '.join(ComponentTypes.get_values())}"
}
