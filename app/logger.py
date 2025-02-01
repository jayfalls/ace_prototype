# DEPENDENCIES
## Built-In
import atexit
import datetime
import inspect
import json
import logging
import os
import re
import signal
import sys
import traceback
## Local
from constants import CustomLogLevels, Defaults, DictKeys, EnvironmentVariables, Folders, TERMINAL_COLOR_CODES


# INITIALISATION
logging.addLevelName(CustomLogLevels.STARTUP, "STARTUP")
logging.addLevelName(CustomLogLevels.STACKTRACE, "STACKTRACE")


# Private
## Formatters
class _JSONFormatter(logging.Formatter):
    def format(self, record: logging.LogRecord):
        log_data: dict[str, str] = {
            DictKeys.TIMESTAMP: datetime.datetime.now(datetime.UTC).isoformat(),
            DictKeys.LEVEL: record.levelname,
            DictKeys.FUNCTION_NAME: f"{record.name}()",
            DictKeys.MESSAGE: record.getMessage(),
        }
        # check if record has attr stacktrace and then add that to dict
        if hasattr(record, DictKeys.STACKTRACE):
            stacktrace: str = re.sub(r"[\x00-\x08\x0b\x0c\x0e-\x1f\x7f-\x9f]", "", record.stacktrace).replace("\n", " | ").replace("'", '').replace('"', '')
            log_data[DictKeys.STACKTRACE] = stacktrace
        return json.dumps(log_data, ensure_ascii=False)

class _HumanReadableFormatter(logging.Formatter):
    def format(self, record: logging.LogRecord) -> str:
        color_code: str = TERMINAL_COLOR_CODES.get(record.levelno, Defaults.TERMINAL_COLOR_CODE)

        log_string = ""
        log_string += datetime.datetime.now(datetime.UTC).strftime("%Y-%m-%d %H:%M:%S")
        log_string += " | "
        log_string += f"{color_code}{record.levelname}{Defaults.TERMINAL_COLOR_CODE}"
        log_string += " | "
        log_string += f"{record.name}()"
        log_string += " | "
        log_string += record.getMessage()

        return log_string


## Logger Class
class _Logger:
    _instance: "_Logger" = None
    _initialised: bool

    def __new__(cls, *args, **kwargs):
        if cls._instance is not None:
            return cls._instance
        cls._instance = super(_Logger, cls).__new__(cls)
        cls._instance._initialised = False
        cls._instance.__init__(*args, **kwargs)
        return cls._instance

    def __init__(self, log_name: str, verbose: bool = False):
        if self._initialised:
            return
        self._initialised = True

        log_file: str = os.path.join(Folders.LOGS, f"{log_name}_{datetime.datetime.now(datetime.UTC).strftime('%Y-%m-%d')}.log")
        log_file = os.path.abspath(log_file)

        self.verbose = verbose

        self.logger: logging.Logger = logging.getLogger()
        self.logger.setLevel(logging.INFO)

        self.logger.handlers: list[logging.Handler] = []
        self.logger.handlers.clear()

        self.file_handler = logging.FileHandler(log_file)
        self.file_handler.setFormatter(_JSONFormatter())
        self.logger.addHandler(self.file_handler)

        self.console_handler: logging.StreamHandler | None = None
        if not self.verbose:
            self.console_handler = logging.StreamHandler(sys.stdout)
            self.console_handler.setFormatter(_HumanReadableFormatter())
            self.logger.addHandler(self.console_handler)

        self._register_shutdown_handlers()

    def _register_shutdown_handlers(self):
        def shutdown_handler():
            self.logger.warning(Defaults.SHUTDOWN_MESSAGE)
            self.file_handler.close()
            if self.console_handler:
                self.console_handler.close()
            self.logger.handlers = []
            logging.shutdown()

        signal.signal(signal.SIGINT, shutdown_handler)
        signal.signal(signal.SIGTERM, shutdown_handler)
        atexit.register(shutdown_handler)

    def _log(self, level: int, message: str, stacktrace: str | None = None):
        caller_frame: inspect.FrameInfo = inspect.currentframe().f_back.f_back
        func_name: str = caller_frame.f_code.co_name

        record = logging.LogRecord(
            name=func_name,
            level=level,
            pathname=caller_frame.f_code.co_filename,
            lineno=caller_frame.f_lineno,
            msg=message,
            args=(),
            exc_info=None
        )
        record.verbose = self.verbose
        if stacktrace:
            record.stacktrace = stacktrace

        self.logger.handle(record)
        if stacktrace and not self.verbose:
            print(stacktrace)
            

    def debug(self, message: str):
        if not self.verbose:
           self._log(logging.DEBUG, message)
    
    def startup(self, message: str):
        self._log(CustomLogLevels.STARTUP, message)

    def info(self, message: str):
        self._log(logging.INFO, message)

    def warn(self, message: str):
        self._log(logging.WARNING, message)

    def error(self, message: str):
        stacktrace: str = "".join(traceback.format_stack()[:-1])
        self._log(logging.ERROR, message, stacktrace)

    def critical(self, message: str):
        stacktrace: str = "".join(traceback.format_stack()[:-1])
        self._log(logging.CRITICAL, message, stacktrace)


# PUBLIC
if not os.environ.get(EnvironmentVariables.LOG_FILE_NAME):
    raise SystemError("You need to initialise the ace logger envs first before using it!")
logger = _Logger(
    log_name=os.environ.get(EnvironmentVariables.LOG_FILE_NAME),
    verbose=bool(os.environ.get(EnvironmentVariables.LOGGER_VERBOSE, False))
)


# EXAMPLES
def example_logs():
    logger.startup("Hello ACE!")
    logger.debug("Dev logs...")
    logger.info("Operations logs...")
    logger.warn("Warning logs...")
    logger.error("Error logs...")
    logger.critical("Critical logs...")
