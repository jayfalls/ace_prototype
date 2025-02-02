# DEPENDENCIES
## Built-In
import os
import importlib
import shutil
## Local
from app.constants.folders import _ENSURED_FOLDERS


# HELPERS
def _cleanup():
    for folder in _ENSURED_FOLDERS:
        if os.path.isdir(folder):
            shutil.rmtree(folder)


# TESTS
def test_ensure_folders_creates_folders():
    """Test that _ensure_json_files creates the necessary JSON files"""
    _cleanup()

    from app.constants import folders
    importlib.reload(folders) # Ensure folders are auto created on module import

    for folder in _ENSURED_FOLDERS:
        if not os.path.isdir(folder):
            raise Exception(f"Folder {folder} does not exist!")
