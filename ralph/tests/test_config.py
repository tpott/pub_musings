"""Tests for config.py"""

import json
import tempfile
import unittest
from pathlib import Path

from ralph.config import ProjectConfig, RalphConfig, load_config


class TestLoadConfig(unittest.TestCase):
    def test_loads_valid_config(self) -> None:
        config_data = {
            "projects": {
                "myproject": {
                    "feedback_script": "myproject/scripts/fetch-feedback.py",
                    "implementation_plan": "myproject/plan.md",
                    "lint_commands": ["cd myproject && lint"],
                    "test_commands": ["cd myproject && test"],
                }
            },
            "defaults": {
                "model": "sonnet",
                "max_iterations": 5,
                "stop_file": "STOP",
            },
        }
        with tempfile.NamedTemporaryFile(mode="w", suffix=".json", delete=False) as f:
            json.dump(config_data, f)
            f.flush()
            config = load_config(Path(f.name))

        self.assertEqual(config.model, "sonnet")
        self.assertEqual(config.max_iterations, 5)
        self.assertEqual(config.stop_file, "STOP")
        self.assertIn("myproject", config.projects)

        proj = config.projects["myproject"]
        self.assertEqual(proj.name, "myproject")
        self.assertEqual(proj.feedback_script, "myproject/scripts/fetch-feedback.py")
        self.assertEqual(proj.implementation_plan, "myproject/plan.md")
        self.assertEqual(proj.lint_commands, ["cd myproject && lint"])
        self.assertEqual(proj.test_commands, ["cd myproject && test"])

    def test_uses_defaults_when_not_specified(self) -> None:
        config_data = {
            "projects": {
                "proj": {
                    "feedback_script": "",
                    "implementation_plan": "",
                }
            }
        }
        with tempfile.NamedTemporaryFile(mode="w", suffix=".json", delete=False) as f:
            json.dump(config_data, f)
            f.flush()
            config = load_config(Path(f.name))

        self.assertEqual(config.model, "opus")
        self.assertEqual(config.max_iterations, 10)
        self.assertEqual(config.stop_file, "STOP_RALPH")

    def test_raises_on_missing_projects_key(self) -> None:
        config_data = {"defaults": {"model": "opus"}}
        with tempfile.NamedTemporaryFile(mode="w", suffix=".json", delete=False) as f:
            json.dump(config_data, f)
            f.flush()
            with self.assertRaises(ValueError):
                load_config(Path(f.name))

    def test_raises_on_missing_file(self) -> None:
        with self.assertRaises(FileNotFoundError):
            load_config(Path("/nonexistent/path/config.json"))

    def test_loads_multiple_projects(self) -> None:
        config_data = {
            "projects": {
                "alpha": {
                    "feedback_script": "alpha/fetch.py",
                    "implementation_plan": "alpha/plan.md",
                },
                "beta": {
                    "feedback_script": "beta/fetch.py",
                    "implementation_plan": "beta/plan.md",
                    "lint_commands": ["cd beta && lint"],
                },
            }
        }
        with tempfile.NamedTemporaryFile(mode="w", suffix=".json", delete=False) as f:
            json.dump(config_data, f)
            f.flush()
            config = load_config(Path(f.name))

        self.assertEqual(len(config.projects), 2)
        self.assertIn("alpha", config.projects)
        self.assertIn("beta", config.projects)
        self.assertEqual(config.projects["beta"].lint_commands, ["cd beta && lint"])
        self.assertEqual(config.projects["alpha"].lint_commands, [])

    def test_project_missing_optional_fields(self) -> None:
        config_data: dict[str, object] = {"projects": {"minimal": {}}}
        with tempfile.NamedTemporaryFile(mode="w", suffix=".json", delete=False) as f:
            json.dump(config_data, f)
            f.flush()
            config = load_config(Path(f.name))

        proj = config.projects["minimal"]
        self.assertEqual(proj.name, "minimal")
        self.assertEqual(proj.feedback_script, "")
        self.assertEqual(proj.implementation_plan, "")
        self.assertEqual(proj.lint_commands, [])
        self.assertEqual(proj.test_commands, [])


class TestProjectConfig(unittest.TestCase):
    def test_dataclass_creation(self) -> None:
        proj = ProjectConfig(
            name="test",
            feedback_script="test/fetch.py",
            implementation_plan="test/plan.md",
            lint_commands=["lint"],
            test_commands=["test"],
        )
        self.assertEqual(proj.name, "test")
        self.assertEqual(proj.feedback_script, "test/fetch.py")

    def test_default_empty_lists(self) -> None:
        proj = ProjectConfig(
            name="test",
            feedback_script="",
            implementation_plan="",
        )
        self.assertEqual(proj.lint_commands, [])
        self.assertEqual(proj.test_commands, [])


class TestRalphConfig(unittest.TestCase):
    def test_dataclass_creation(self) -> None:
        config = RalphConfig(
            projects={},
            model="sonnet",
            max_iterations=5,
            stop_file="STOP",
        )
        self.assertEqual(config.model, "sonnet")
        self.assertEqual(config.max_iterations, 5)
        self.assertEqual(config.stop_file, "STOP")

    def test_default_values(self) -> None:
        config = RalphConfig(projects={})
        self.assertEqual(config.model, "opus")
        self.assertEqual(config.max_iterations, 10)
        self.assertEqual(config.stop_file, "STOP_RALPH")


if __name__ == "__main__":
    unittest.main()
