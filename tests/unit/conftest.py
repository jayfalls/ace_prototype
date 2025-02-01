# DEPENDENCIES
## Built-In
from pathlib import Path
import shutil
import sys


PROJECT_ROOT = Path(__file__).parent.parent.parent
sys.path.insert(0, str(PROJECT_ROOT / "app"))
sys.path.insert(0, str(PROJECT_ROOT / "tests"))


# POST TESTING CLEANUP
from app.constants.folders import _ENSURED_FOLDERS
def pytest_sessionfinish(session, exitstatus):
    """Clean up after the test session."""
    print("\nCleaning up test directories...")
    for folder in _ENSURED_FOLDERS:
        folder_path = Path(folder)
        if folder_path.exists():
            try:
                shutil.rmtree(folder_path)
                print(f"Removed: {folder_path}")
            except PermissionError:
                print(f"Warning: Could not remove {folder_path} - Permission denied")
            except Exception as e:
                print(f"Warning: Could not remove {folder_path} - {str(e)}")
    if session.testsfailed > 0:
        print("Tests failed!")
        # Force exit status to 1 if any tests failed
        session.exitstatus = 1
    return session.exitstatus
