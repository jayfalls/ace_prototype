# DEPENDENCIES
## Built-In
import sys
from pathlib import Path


PROJECT_ROOT = Path(__file__).parent.parent.parent
sys.path.insert(0, str(PROJECT_ROOT / "app"))
sys.path.insert(0, str(PROJECT_ROOT / "tests"))
