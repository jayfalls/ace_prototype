# DEPENDENCIES
## Third-Party
import pytest
## Local
from app.constants.base_enum import BaseEnum

def test_type_checking():
    """Test that invalid types raise a TypeError."""
    class ValidEnum(BaseEnum):
        VALID_ATTR = "value" # String
        VALID_ATTR2 = 123 # Integer
        VALID_ATTR3 = 0.2 # Float

    # Test non type hinted attributes
    with pytest.raises(TypeError):
        class InvalidEnum(BaseEnum):
            INVALID_ATTR = {"key": "value"}
            INVALID_ATTR2 = True
    
    # Test type hinted attributes
    with pytest.raises(TypeError):
        class InvalidEnum(BaseEnum):
            INVALID_ATTR: dict = {"key": "value"}
            INVALID_ATTR2: bool = True

def test_get_dict():
    """Test that get_dict() returns the correct dictionary."""
    class TestEnum(BaseEnum):
        ATTR1 = "value1"
        ATTR2 = "value2"
        _PRIVATE_ATTR = "private"

    expected_dict: dict[str, str] = {"ATTR1": "value1", "ATTR2": "value2"}
    assert TestEnum.get_dict() == expected_dict, "get_dict() should return the correct dictionary"

def test_get_tuple():
    """Test that get_tuple() returns the correct tuple."""
    class TestEnum(BaseEnum):
        ATTR1 = "value1"
        ATTR2 = "value2"

    expected_tuple: tuple[str, str] = ("value1", "value2")
    assert TestEnum.get_tuple() == expected_tuple, "get_tuple() should return the correct tuple"

def test_get_frozenset():
    """Test that get_frozenset() returns the correct frozenset."""
    class TestEnum(BaseEnum):
        ATTR1 = "value1"
        ATTR2 = "value2"

    expected_set: frozenset[str] = frozenset(["value1", "value2"])
    assert TestEnum.get_frozenset() == expected_set, "get_frozenset() should return the correct frozenset"
