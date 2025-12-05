# serve_config.py

import json
import os
from typing import Any, Dict


PAYMENT_LINK_EXPIRY_HOURS = 24


def getConfig() -> Dict[str, Any]:
    """
    Load and return the server configuration from the SERVE_CONFIG environment variable.
    The SERVE_CONFIG env var should point to a JSON file path.

    Expected structure:
    {
      "facebook_app_id": "your_app_id_here",
      "facebook_app_secret": "app_secret_here",
      "webhook_cert_file": "path/to/cert.pem",
      "webhook_key_file": "path/to/key.pem",
      "open_api_key": "your_api_key_here",
      "pages": [
        {
          "page_id": "your_page_id_here",
          "page_token": "your_expiring_or_non_expiring_token",
          "last_run": 123456,
        },
      ],
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
    assert "facebook_app_id" in config, "Config missing required field: facebook_app_id"
    assert "facebook_app_secret" in config, (
        "Config missing required field: facebook_app_secret"
    )
    assert "open_api_key" in config, "Config missing required field: open_api_key"
    assert "pages" in config, "Config missing required field: pages"
    assert isinstance(config["pages"], list), "Config 'pages' must be an array"

    # Validate each page has required fields
    for i, page in enumerate(config["pages"]):
        assert "page_id" in page, f"Config pages[{i}] missing required field: page_id"
        assert "page_token" in page, (
            f"Config pages[{i}] missing required field: page_token"
        )
        assert "last_run" in page, f"Config pages[{i}] missing required field: last_run"

    # Validate payment session fields
    assert "webhook_hostname" in config, (
        "Config missing required field: webhook_hostname"
    )
    assert "payment_sessions_dir" in config, (
        "Config missing required field: payment_sessions_dir"
    )
    assert "users_dir" in config, "Config missing required field: users_dir"
    assert "conversations_dir" in config, (
        "Config missing required field: conversations_dir"
    )

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
