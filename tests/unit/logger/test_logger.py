# DEPENDENCIES
## Built-In
import json
import logging
import os
import signal
import sys
from unittest import mock
## Third-Party
import pytest
## Local
from app.constants import DictKeys, Defaults, EnvironmentVariables, Folders
from tests.unit.constants import TestingConfigs, Modules


# HELPERS
def _get_log_files() -> list[str]:
    return [
        os.path.join(Folders.LOGS, f)
        for f in os.listdir(Folders.LOGS)
        if f.endswith('.log')
    ]

def _cleanup_module():
    if Modules.LOGGER in sys.modules:
        del sys.modules[Modules.LOGGER]
    from app.logger import _Logger
    _Logger._instance = None
    if Modules.LOGGER in sys.modules:
        del sys.modules[Modules.LOGGER]

def _cleanup_log_files():
    for f in _get_log_files():
        os.remove(f)




# TESTS
def test_singleton_enforcement(cleanup):
    """Ensure only one logger instance exists"""
    from app.logger import _Logger
    logger1 = _Logger(TestingConfigs.LOG_FILE_NAME, False)
    logger2 = _Logger(TestingConfigs.LOG_FILE_NAME, False)
    assert logger1 is logger2

def test_missing_environment_variables(cleanup):
    """Test required environment variables validation"""
    with mock.patch.dict(os.environ, clear=True):
        if Modules.LOGGER in sys.modules:
            del sys.modules[Modules.LOGGER]
        with pytest.raises(SystemError):
            from app.logger import logger  # noqa: F401

def test_file_handler_creation(cleanup):
    """Verify log file is created in correct location"""
    from app.logger import logger
    
    logger.info("Test message")
    logger.error("Test message")
    
    assert len(_get_log_files()) > 0, "No log file created in test directory"
    
    with open(_get_log_files()[0]) as f:
        content = f.read()
        assert "Test message" in content, "Log message missing from file"

def test_verbose_mode_handler_config(monkeypatch, cleanup):
    """Test handler configuration based on verbosity"""
    monkeypatch.setenv(EnvironmentVariables.LOGGER_VERBOSE, TestingConfigs.VERBOSE_ENABLED)
    from app.logger import logger

    file_handlers: logging.Handler = [handler for handler in logger.logger.handlers if isinstance(handler, logging.FileHandler)]
    console_handlers: logging.Handler = [handler for handler in logger.logger.handlers if isinstance(handler, logging.StreamHandler) and handler == logger.console_handler]

    assert len(file_handlers) == 1, "Should have 1 FileHandler"
    assert len(console_handlers) != 0, "Should have 0 ConsoleHandlers in verbose mode"

def test_non_verbose_mode_handler_config(monkeypatch, cleanup):
    """Test handler configuration based on verbosity"""
    monkeypatch.setenv(EnvironmentVariables.LOGGER_VERBOSE, TestingConfigs.VERBOSE_DISABLED)
    from app.logger import logger

    file_handlers: logging.Handler = [handler for handler in logger.logger.handlers if isinstance(handler, logging.FileHandler)]
    console_handlers: logging.Handler = [handler for handler in logger.logger.handlers if isinstance(handler, logging.StreamHandler) and handler == logger.console_handler]

    assert len(file_handlers) == 1, "Should have 1 FileHandler"
    assert len(console_handlers) == 1, "Should have 1 ConsoleHandlers in non verbose mode"

def test_shutdown_cleanup(cleanup):
    """Verify handlers close properly on shutdown"""
    from app.logger import logger

    initial_handlers: int = len(logger.logger.handlers)

    logger._register_shutdown_handlers()
    signal_handlers = signal.getsignal(signal.SIGTERM)
    if isinstance(signal_handlers, list):
        signal_handlers[0]()
    else:
        signal_handlers()

    assert len(logger.logger.handlers) < initial_handlers

    with open(_get_log_files()[0]) as f:
        content: dict = json.loads(f.read().strip())
        assert content[DictKeys.MESSAGE] == Defaults.SHUTDOWN_MESSAGE
        assert content[DictKeys.LEVEL] == "WARNING"

def test_debug_logging_non_verbose_mode(monkeypatch, cleanup):
    """Debug messages should NOT appear in logs when verbose=False"""
    monkeypatch.setenv(EnvironmentVariables.LOGGER_VERBOSE, TestingConfigs.VERBOSE_DISABLED)
    from app.logger import logger

    logger.debug("Debug test in non-verbose mode")

    with open(_get_log_files()[0]) as f:
        content: dict = json.loads(f.read().strip())
        assert content[DictKeys.MESSAGE] == "Debug test in non-verbose mode"

def test_debug_logging_verbose_mode(monkeypatch, cleanup):
    """Debug messages should appear in logs when verbose=True"""
    monkeypatch.setenv(EnvironmentVariables.LOGGER_VERBOSE, TestingConfigs.VERBOSE_ENABLED)
    
    from app.logger import logger

    logger.debug("Debug test in verbose mode")
    logger.info("Info test in verbose mode")

    with open(_get_log_files()[0]) as f:
        content: dict = json.loads(f.read().strip())
        assert content[DictKeys.MESSAGE] != "Debug test in verbose mode"

def test_stacktrace_on_error(cleanup):
    """Verify stacktrace is logged on error"""
    from app.logger import logger

    logger.error("Error test")

    with open(_get_log_files()[0]) as f:
        content: dict = json.loads(f.read().strip())
        assert content[DictKeys.MESSAGE] == "Error test"
        assert content[DictKeys.STACKTRACE]

def test_stacktrace_on_critical(cleanup):
    """Verify stacktrace is logged on critical error"""
    from app.logger import logger

    logger.error("Critical test")
    
    with open(_get_log_files()[0]) as f:
        content: dict = json.loads(f.read().strip())
        assert content[DictKeys.MESSAGE] == "Critical test"
        assert content[DictKeys.STACKTRACE]

def test_startup_message(cleanup):
    """Verify startup message is logged"""
    from app.logger import logger

    logger.startup("Startup test")

    with open(_get_log_files()[0]) as f:
        content: dict = json.loads(f.read().strip())
        assert content[DictKeys.MESSAGE] == "Startup test"
        assert content[DictKeys.LEVEL] == "STARTUP"

def test_info_message(cleanup):
    """Verify info message is logged"""
    from app.logger import logger

    logger.info("Info test")

    with open(_get_log_files()[0]) as f:
        content: dict = json.loads(f.read().strip())
        assert content[DictKeys.MESSAGE] == "Info test"
        assert content[DictKeys.LEVEL] == "INFO"


# CLEANUP
@pytest.fixture(autouse=True)
def cleanup():
    _cleanup_module()
    _cleanup_log_files()
    yield
    _cleanup_module()
