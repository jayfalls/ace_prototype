# DEPENDENCIES
## Built-In
from datetime import datetime
import json
import logging
import sys
import traceback


class Logger:
    def __init__(self, name: str, log_file: str = "app.log"):
        self.logger = logging.getLogger(name)
        self.logger.setLevel(logging.INFO)
        self.logger.handlers = []  # Clear existing handlers
        
        # Simple JSON formatter
        class JSONFormatter(logging.Formatter):
            def format(self, record):
                log_data = {
                    "timestamp": datetime.utcnow().isoformat(),
                    "level": record.levelname,
                    "message": record.getMessage()
                }
                if hasattr(record, 'stack_trace'):
                    log_data['stack_trace'] = record.stack_trace
                return json.dumps(log_data)
        
        # File handler
        file_handler = logging.FileHandler(log_file)
        file_handler.setFormatter(JSONFormatter())
        self.logger.addHandler(file_handler)
        
        # Console handler
        console_handler = logging.StreamHandler(sys.stdout)
        console_handler.setFormatter(JSONFormatter())
        self.logger.addHandler(console_handler)
    
    def _log(self, level: int, message: str, stack_trace: str = None):
        record = logging.LogRecord(
            name=self.logger.name,
            level=level,
            pathname="",
            lineno=0,
            msg=message,
            args=(),
            exc_info=None
        )
        if stack_trace:
            record.stack_trace = stack_trace
        
        for handler in self.logger.handlers:
            handler.handle(record)
    
    def info(self, message: str):
        self._log(logging.INFO, message)
    
    def error(self, message: str):
        self._log(logging.ERROR, message)
    
    def stack(self, message: str = "Stack trace"):
        stack_trace = ''.join(traceback.format_stack()[:-1])  # Exclude this call
        self._log(logging.ERROR, message, stack_trace)