# serve_config.py

import json
import os
from pathlib import Path
from typing import Any, Dict, Optional


PAYMENT_LINK_EXPIRY_HOURS = 24


def get_config() -> Dict[str, Any]:
    """
    Load and return the server configuration from the SERVE_CONFIG environment variable.
    The SERVE_CONFIG env var should point to a JSON file path.

    Expected structure:
    {
      "open_api_key": "your_api_key_here",
      "anthropic_api_key": "sk-ant-...",
      "hosts": {
        "devbox": {"hostname": "devbox.example.com", "user": "trevor"},
        "prod": {"hostname": "prod.example.com", "user": "deploy"}
      },
      "default_host": "devbox"
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

    # Validate required fields for Anthropic/DevAgent mode
    assert "anthropic_api_key" in config, (
        "Config missing required field: anthropic_api_key"
    )

    return config


def get_data_dir() -> Path:
    """Return the data directory path (same directory as config file)."""
    config_path = os.environ.get("SERVE_CONFIG")
    assert config_path is not None, "Missing env var: SERVE_CONFIG"
    return Path(config_path).parent


def get_hosts(config: Optional[Dict[str, Any]] = None) -> Dict[str, Dict[str, str]]:
    """Return the configured hosts dictionary."""
    if config is None:
        config = get_config()
    return config.get("hosts", {})


def get_default_host(config: Optional[Dict[str, Any]] = None) -> Optional[str]:
    """Return the default host name."""
    if config is None:
        config = get_config()
    return config.get("default_host")


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
