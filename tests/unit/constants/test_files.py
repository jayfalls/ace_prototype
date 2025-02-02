# DEPENDENCIES
## Built-In
import os
import importlib
import json
## Third-Party
import pytest
import yaml
## Local
from app.constants.files import _ENSURE_JSON_FILES, Files, setup_user_deployment_file


# HELPERS
@pytest.fixture
def _ensure_app_directory():
    os.chdir("./app")
    yield
    os.chdir("..")

def _cleanup():
    for file in _ENSURE_JSON_FILES:
        if os.path.isfile(file):
            os.remove(file)


# TESTS
def test_setup_user_deployment_file_creates_valid_yaml_file(_ensure_app_directory):
    """Test that setup_user_deployment_file creates a valid deployment.yaml file without placeholders"""
    def check_deployment_file(dev: bool):
        env_keyword: str = "dev" if dev else "prod"
        setup_user_deployment_file(dev=dev)
        if not os.path.isfile(Files.USER_DEPLOYMENT_FILE):
            raise Exception("user_deployment.yaml file not created")
        with open(Files.USER_DEPLOYMENT_FILE, "r") as f:
            content: str = f.read()
            assert "{{" not in content and "}}" not in content, "All placeholders should be replaced"
            assert env_keyword in content, f"Environment should be set to {env_keyword} when creating deployment file with dev={dev}"
            yaml.safe_load(content)
    check_deployment_file(dev=True)
    check_deployment_file(dev=False)

def test_ensure_json_files_creates_valid_json_files():
    """Test that _ensure_json_files creates the necessary JSON files"""
    _cleanup()

    from app import constants
    importlib.reload(constants) # Ensure folders & files are auto created on module import
    from app.constants import files
    importlib.reload(files) # Ensure files are auto created on module import

    for file in _ENSURE_JSON_FILES:
        with open(file, "r") as f:
            content: str = f.read()
            json_content: dict = json.loads(content)
            assert content.strip() == "{}", "JSON file should be an empty dictionary"
            assert json_content == {}, "JSON file should be an empty dictionary"
