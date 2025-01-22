# DEPENDENCIES
## Built-In
import json
import logging
import os
import shutil
import signal
import sys
from unittest import mock
## Third-Party
import pytest


def test_singleton_enforcement():
    """Ensure only one logger instance exists"""
    with mock.patch.dict(os.environ, {
        "ACE_LOGGER_FILE_NAME": "test",
        "ACE_LOGGER_VERBOSE": "False"
    }):
        from app.logger.logger import _Logger
        logger1 = _Logger("test", False)
        logger2 = _Logger("test", False)
        assert logger1 is logger2

def test_missing_environment_variables():
    """Test required environment variables validation"""
    with mock.patch.dict(os.environ, clear=True):
        # Force reload module
        if 'app.logger.logger' in sys.modules:
            del sys.modules['app.logger.logger']
        with pytest.raises(SystemError):
            from app.logger.logger import logger  # noqa: F401

def test_file_handler_creation(tmp_path, monkeypatch):
    """Verify log file is created in correct location"""
    # Set up test environment
    test_log_dir = tmp_path / "logs"
    test_log_dir.mkdir()
    
    # Set environment variables
    monkeypatch.setenv("ACE_LOGGER_FILE_NAME", "test_log")
    monkeypatch.setenv("ACE_LOGGER_VERBOSE", "")
    
    # Change current directory to tmp_path so that _LOG_FOLDER is tmp_path/logs
    monkeypatch.chdir(tmp_path)
    
    # Force reload module to apply patches, reset singleton state and import fresh instance
    if 'app.logger.logger' in sys.modules:
        del sys.modules['app.logger.logger']
    from app.logger.logger import _Logger
    _Logger._instance = None
    from app.logger.logger import logger
    
    # Test log message
    logger.info("Test message")
    
    # Find generated log files
    log_files = list(test_log_dir.glob("test_log_*.log"))
    
    # Verify file creation
    assert len(log_files) == 1, "No log file created in test directory"
    
    # Verify content
    with open(log_files[0]) as f:
        content = f.read()
        assert "Test message" in content, "Log message missing from file"

def test_verbose_mode_handler_config():
    """Test handler configuration based on verbosity"""
    with mock.patch.dict(os.environ, {
        "ACE_LOGGER_FILE_NAME": "test",
        "ACE_LOGGER_VERBOSE": "."
    }):
        # Force reload module to apply patches, reset singleton state and import fresh instance
        if 'app.logger.logger' in sys.modules:
            del sys.modules['app.logger.logger']
        from app.logger.logger import _Logger
        _Logger._instance = None
        from app.logger.logger import logger
    
        # Check handlers
        file_handlers = [h for h in logger.logger.handlers if isinstance(h, logging.FileHandler)]
        console_handlers = [h for h in logger.logger.handlers if isinstance(h, logging.StreamHandler) and h == logger.console_handler]
    
        assert len(file_handlers) == 1, "Should have 1 FileHandler"
        assert len(console_handlers) == 0, "Should have 0 ConsoleHandlers in verbose mode"

def test_shutdown_cleanup(tmp_path):
    """Verify handlers close properly on shutdown"""
    test_log_dir = tmp_path / "logs"
    test_log_dir.mkdir(exist_ok=True)
    
    with mock.patch.dict(os.environ, {
        "ACE_LOGGER_FILE_NAME": "test",
        "ACE_LOGGER_VERBOSE": ""
    }):
        with mock.patch("app.logger.logger._LOG_FOLDER", str(test_log_dir)):
            if 'app.logger.logger' in sys.modules:
                del sys.modules['app.logger.logger']
            from app.logger.logger import logger
            
            # Store initial handler count
            initial_handlers = len(logger.logger.handlers)
            
            # Call the shutdown handler function directly
            logger._register_shutdown_handlers()
            # Get the function we registered
            signal_handlers = signal.getsignal(signal.SIGTERM)
            if isinstance(signal_handlers, list):
                signal_handlers[0]()
            else:
                signal_handlers()
            
            # Verify handlers were removed
            assert len(logger.logger.handlers) < initial_handlers

def test_debug_logging_non_verbose_mode(tmp_path, monkeypatch):
    """Debug messages should NOT appear in logs when verbose=False"""
    # Setup
    test_log_dir = tmp_path / "logs"
    test_log_dir.mkdir()
    monkeypatch.setenv("ACE_LOGGER_FILE_NAME", "test")
    monkeypatch.setenv("ACE_LOGGER_VERBOSE", "")
    monkeypatch.chdir(tmp_path)
    
    # Reset logger
    if 'app.logger.logger' in sys.modules:
        del sys.modules['app.logger.logger']
    from app.logger.logger import _Logger
    _Logger._instance = None
    from app.logger.logger import logger
    
    # Test
    logger.debug("Debug test in non-verbose mode")
    logger.info("Info test in non-verbose mode")
    
    # Verify
    log_files = list(test_log_dir.glob("test_*.log"))
    with open(log_files[0]) as f:
        content = json.loads(f.read().strip())
        for value in content.values():
            assert value != "Debug test in non-verbose mode"

def test_debug_logging_verbose_mode(tmp_path, monkeypatch):
    """Debug messages should appear in logs when verbose=True"""
    # Setup
    test_log_dir = tmp_path / "logs"
    test_log_dir.mkdir()
    monkeypatch.setenv("ACE_LOGGER_FILE_NAME", "test")
    monkeypatch.setenv("ACE_LOGGER_VERBOSE", ".")
    monkeypatch.chdir(tmp_path)
    
    # Reset logger
    if 'app.logger.logger' in sys.modules:
        del sys.modules['app.logger.logger']
    from app.logger.logger import _Logger
    _Logger._instance = None
    from app.logger.logger import logger
    
    # Test
    logger.debug("Debug test in verbose mode")
    
    # Verify
    log_files = list(test_log_dir.glob("test_*.log"))
    with open(log_files[0]) as f:
        content = json.loads(f.read().strip())
        assert content["message"] == "Debug test in verbose mode"

@pytest.fixture(autouse=True)
def cleanup_log_folder():
    log_folder: str = os.path.abspath("logs")
    if os.path.exists(log_folder):
        shutil.rmtree(log_folder)
