# DEPENDENCIES
## Built-In
import atexit
from datetime import datetime
import inspect
import json
import logging
import os
import signal
import sys
import traceback


# CONSTANTS
_COLOR_CODES: dict[int, str] = {
    logging.DEBUG: "\033[36m",    # Cyan
    logging.INFO: "\033[32m",     # Green
    logging.WARNING: "\033[32m",  # Orange
    logging.ERROR: "\033[31m",    # Red
    logging.CRITICAL: "\033[41m", # Red background
}
_DEFAULT_COLOR_CODE: str = "\033[0m"  # Default color
_DEFAULT_LOG_FILE_NAME: str = "startup"
_LOG_FOLDER: str = "logs"
class _DictKeys:
    FUNCTION_NAME: str = "function_name"
    LEVEL: str = "level"
    MESSAGE: str = "message"
    TIMESTAMP: str = "timestamp"
class _EnvironmentVariables:
    LOGGER_NAME: str = "ACE_LOGGER_FILE_NAME"
    LOGGER_VERBOSE: str = "ACE_LOGGER_VERBOSE"


# Private
## Formatters
class _JSONFormatter(logging.Formatter):
    def format(self, record: logging.LogRecord):
        log_data: dict[str, str] = {
            _DictKeys.TIMESTAMP: datetime.utcnow().isoformat(),
            _DictKeys.LEVEL: record.levelname,
            _DictKeys.FUNCTION_NAME: f"{record.name}()",
            _DictKeys.MESSAGE: record.getMessage(),
        }
        return json.dumps(log_data)

class _HumanReadableFormatter(logging.Formatter):
    def format(self, record: logging.LogRecord) -> str:
        color_code: str = _COLOR_CODES.get(record.levelno, _DEFAULT_COLOR_CODE)

        log_string = ""
        log_string += datetime.utcnow().strftime("%Y-%m-%d %H:%M:%S")
        log_string += " | "
        log_string += f"{color_code}{record.levelname}{_DEFAULT_COLOR_CODE}"
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

        os.makedirs(_LOG_FOLDER, exist_ok=True)
        log_file = f"{_LOG_FOLDER}/{log_name}_{datetime.utcnow().strftime('%Y-%m-%d')}.log"

        self.verbose = verbose

        self.logger: logging.Logger = logging.getLogger()
        self.logger.setLevel(logging.INFO)

        self.logger.handlers: list[logging.Handler] = []

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
            self.logger.warning("Shutting down logger...")
            self.file_handler.close()
            if self.console_handler:
                self.console_handler.close()
            self.logger.handlers = []
            logging.shutdown()

        signal.signal(signal.SIGINT, shutdown_handler)
        signal.signal(signal.SIGTERM, shutdown_handler)
        atexit.register(shutdown_handler)

    def _log(self, level: int, message: str, stack_trace: str | None = None):
        caller_frame: inspect.FrameInfo = inspect.currentframe().f_back.f_back
        func_name: str = caller_frame.f_code.co_name

        record = logging.LogRecord(
            name=func_name,
            level=level,
            pathname=caller_frame.f_code.co_filename,
            lineno=caller_frame.f_lineno,
            msg=message,
            args=(),
            exc_info=None,
        )
        if self.verbose and level == logging.DEBUG:
            return
        self.logger.handle(record)

        if stack_trace:
            self.logger.error(stack_trace)

    def debug(self, message: str):
        self._log(logging.DEBUG, message)

    def info(self, message: str):
        self._log(logging.INFO, message)

    def warn(self, message: str):
        self._log(logging.WARNING, message)

    def error(self, message: str):
        stack_trace = "".join(traceback.format_stack()[:-1])
        self._log(logging.ERROR, message, stack_trace)

    def critical(self, message: str):
        stack_trace = "".join(traceback.format_stack()[:-1])
        self._log(logging.CRITICAL, message, stack_trace)


# PUBLIC
if not os.environ.get(_EnvironmentVariables.LOGGER_NAME):
    raise SystemError("You need to initialise the ace logger envs first before using it!")
logger = _Logger(
    log_name=os.environ.get(_EnvironmentVariables.LOGGER_NAME),
    verbose=bool(os.environ.get(_EnvironmentVariables.LOGGER_VERBOSE, False))
)
