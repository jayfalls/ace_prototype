# DEPENDENCIES
## Built-In
from unittest.mock import patch, MagicMock
## Third-Party
import pytest
## Local
from app.component import run_component
from app.constants import Components


def test_run_component_no_environment_specified(monkeypatch):
    """Test run_component with no environment specified"""
    with patch("app.component.logger.critical") as mock_critical:
        with pytest.raises(SystemExit):
            run_component()
            mock_critical.assert_called_once_with("You must select a environment, either --dev or --prod!")

def test_run_component_multiple_components(monkeypatch):
    """Test run_component with multiple components specified"""
    with patch("argparse.ArgumentParser.parse_args") as mock_parse_args:
        mock_args = MagicMock()
        mock_args.dev = True
        mock_args[Components.CONTROLLER] = True
        mock_args[Components.UI] = True
        mock_parse_args.return_value = mock_args
        
        # Mock logger.critical and exit
        with patch("app.component.logger.critical") as mock_critical:
            with pytest.raises(SystemExit):
                run_component()
                mock_critical.assert_called_once_with("You can only start one component at a time!")

def test_run_component_invalid_component(monkeypatch):
    """Test run_component with an invalid component specified."""
    invalid_component: str = "invalid_component"
    with patch("argparse.ArgumentParser.parse_args") as mock_parse_args:
        mock_args = MagicMock()
        mock_args.dev = True
        mock_args[invalid_component] = True
        mock_parse_args.return_value = mock_args
        
        # Mock logger.critical and exit
        with patch("app.component.logger.critical") as mock_critical:
            with pytest.raises(SystemExit):
                run_component()
                mock_critical.assert_called_once_with(f"{invalid_component} is not a valid component!")

def test_run_component_no_component_selected(monkeypatch):
    """Test run_component with no component selected."""
    with patch("argparse.ArgumentParser.parse_args") as mock_parse_args:
        mock_args = MagicMock()
        mock_args.dev = True
        # No component flags set
        mock_parse_args.return_value = mock_args

        with patch("app.component.logger.critical") as mock_critical:
            with pytest.raises(SystemExit):
                run_component()
                mock_critical.assert_called_once_with("You must select a component to start!")
