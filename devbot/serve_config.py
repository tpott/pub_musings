# serve_config.py

import json
import os
from typing import Any, Dict


PAYMENT_LINK_EXPIRY_HOURS = 24


def get_config() -> Dict[str, Any]:
    """
    Load and return the server configuration from the SERVE_CONFIG environment variable.
    The SERVE_CONFIG env var should point to a JSON file path.

    Expected structure:
    {
      "open_api_key": "your_api_key_here",
    }

    Returns:
        Dict containing the configuration

    Raises:
        AssertionError: If SERVE_CONFIG env var is not set
        FileNotFoundError: If config file doesn't exist
        json.JSONDecodeError: If config file contains invalid JSON
    """
    config_path = os.environ.get("SERVE_CONFIG")
    assert config_path is not None, "Missing env var: SERVE_CONFIG"

    with open(config_path, "r") as f:
        config = json.load(f)

    # Validate required fields
    assert "open_api_key" in config, "Config missing required field: open_api_key"

    return config


def saveConfig(config: Dict[str, Any]) -> None:
    """
    Save the configuration back to the file specified by SERVE_CONFIG.

    Args:
        config: The configuration dict to save

    Raises:
        AssertionError: If SERVE_CONFIG env var is not set
    """
    config_path = os.environ.get("SERVE_CONFIG")
    assert config_path is not None, "Missing env var: SERVE_CONFIG"

    with open(config_path, "w") as f:
        json.dump(config, f, indent=2)
