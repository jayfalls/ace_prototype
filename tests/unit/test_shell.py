# DEPENDENCIES
## Third-Party
import pytest
## Local
from app.shell import execute_shell, exec_check_exists


@pytest.mark.parametrize("command,expected_output", [
    ("echo 'test'", "test\n"),
    ("ls -a", ".\n..\n")
])
def test_execute_shell(command, expected_output, caplog):
    """Test the execute_shell function with various commands."""
    output: str = execute_shell(command)
    assert expected_output in output, f"Expected '{expected_output}' in output, but got '{output}'"
    with pytest.raises(SystemExit):
        execute_shell("invalid_command", ignore_error=False, _testing=True)
    error_output: str = execute_shell("invalid_command", ignore_error=True)
    assert any(msg in error_output for msg in [
        "invalid_command: command not found",
        "/bin/sh: 1: invalid_command: not found"
    ]), "Error message should be printed"

def test_exec_check_exists():
    """Test the exec_check_exists function."""
    result = exec_check_exists("ls -a", ".")
    assert result is True, "Expected True, but got False"
    result = exec_check_exists("ls -a", "non_existent_file")
    assert result is False, "Expected False, but got True"

def test_should_print_result(capsys):
    """Test the should_print_result parameter."""
    execute_shell("echo 'test'", should_print_result=True)
    captured = capsys.readouterr()
    assert "test\n" in captured.out, "Output should be printed"
    execute_shell("echo 'test'", should_print_result=False)
    captured = capsys.readouterr()
    assert "test\n" not in captured.out, "Output should not be printed"
