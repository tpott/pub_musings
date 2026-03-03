"""Configuration system for multi-project Ralph orchestrator."""

import json
from dataclasses import dataclass, field
from pathlib import Path


@dataclass
class ProjectConfig:
    """Configuration for a single project managed by Ralph."""

    name: str
    feedback_script: str
    implementation_plan: str
    lint_commands: list[str] = field(default_factory=list)
    test_commands: list[str] = field(default_factory=list)


@dataclass
class RalphConfig:
    """Top-level Ralph configuration."""

    projects: dict[str, ProjectConfig]
    model: str = "opus"
    max_iterations: int = 10
    stop_file: str = "STOP_RALPH"


def load_config(path: Path) -> RalphConfig:
    """Load Ralph configuration from a JSON file.

    Args:
        path: Path to the JSON config file.

    Returns:
        Parsed RalphConfig.

    Raises:
        FileNotFoundError: If the config file doesn't exist.
        ValueError: If the config file is invalid.
    """
    with open(path) as f:
        raw = json.load(f)

    if "projects" not in raw:
        raise ValueError(f"Config file {path} missing 'projects' key")

    defaults = raw.get("defaults", {})
    model = defaults.get("model", "opus")
    max_iterations = defaults.get("max_iterations", 10)
    stop_file = defaults.get("stop_file", "STOP_RALPH")

    projects: dict[str, ProjectConfig] = {}
    for name, proj_raw in raw["projects"].items():
        projects[name] = ProjectConfig(
            name=name,
            feedback_script=proj_raw.get("feedback_script", ""),
            implementation_plan=proj_raw.get("implementation_plan", ""),
            lint_commands=proj_raw.get("lint_commands", []),
            test_commands=proj_raw.get("test_commands", []),
        )

    return RalphConfig(
        projects=projects,
        model=model,
        max_iterations=max_iterations,
        stop_file=stop_file,
    )
