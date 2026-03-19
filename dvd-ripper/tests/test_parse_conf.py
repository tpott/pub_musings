"""Tests for rip.parse_conf."""

import os
import tempfile
import textwrap
import unittest
from pathlib import Path

from rip import parse_conf


class TestParseConf(unittest.TestCase):

    def _write_conf(self, content):
        """Write config content to a temp file and return its path."""
        f = tempfile.NamedTemporaryFile(mode="w", suffix=".conf", delete=False)
        f.write(textwrap.dedent(content))
        f.close()
        self.addCleanup(os.unlink, f.name)
        return f.name

    def test_basic_key_value(self):
        conf = self._write_conf('''\
            FOO="bar"
            BAZ="qux"
        ''')
        result = parse_conf(conf)
        self.assertEqual(result["FOO"], "bar")
        self.assertEqual(result["BAZ"], "qux")

    def test_single_quotes(self):
        conf = self._write_conf("""\
            FOO='bar'
        """)
        result = parse_conf(conf)
        self.assertEqual(result["FOO"], "bar")

    def test_unquoted_value(self):
        conf = self._write_conf("""\
            FOO=bar
        """)
        result = parse_conf(conf)
        self.assertEqual(result["FOO"], "bar")

    def test_comments_and_blank_lines(self):
        conf = self._write_conf('''\
            # This is a comment
            FOO="bar"

            # Another comment
            BAZ="qux"
        ''')
        result = parse_conf(conf)
        self.assertEqual(result, {"FOO": "bar", "BAZ": "qux"})

    def test_dollar_var_expansion_from_config(self):
        conf = self._write_conf('''\
            BASE="/opt"
            FULL="$BASE/data"
        ''')
        result = parse_conf(conf)
        self.assertEqual(result["FULL"], "/opt/data")

    def test_dollar_brace_expansion_from_config(self):
        conf = self._write_conf('''\
            BASE="/opt"
            FULL="${BASE}/data"
        ''')
        result = parse_conf(conf)
        self.assertEqual(result["FULL"], "/opt/data")

    def test_env_var_expansion(self):
        old = os.environ.get("HOME")
        os.environ["HOME"] = "/home/testuser"
        try:
            conf = self._write_conf('''\
                BIN="$HOME/.local/bin/tool"
            ''')
            result = parse_conf(conf)
            self.assertEqual(result["BIN"], "/home/testuser/.local/bin/tool")
        finally:
            if old is not None:
                os.environ["HOME"] = old

    def test_config_var_takes_precedence_over_env(self):
        os.environ["FOO"] = "from_env"
        try:
            conf = self._write_conf('''\
                FOO="from_conf"
                BAR="$FOO/sub"
            ''')
            result = parse_conf(conf)
            self.assertEqual(result["BAR"], "from_conf/sub")
        finally:
            del os.environ["FOO"]

    def test_unknown_var_left_as_is(self):
        os.environ.pop("NONEXISTENT_VAR_XYZ", None)
        conf = self._write_conf('''\
            FOO="$NONEXISTENT_VAR_XYZ"
        ''')
        result = parse_conf(conf)
        self.assertEqual(result["FOO"], "$NONEXISTENT_VAR_XYZ")

    def test_real_conf_example(self):
        """Parse the actual rip.conf.example and verify key fields."""
        example = Path(__file__).resolve().parent.parent / "rip.conf.example"
        result = parse_conf(example)
        self.assertEqual(result["RIP_DIR"], "/home/youruser/dvd-ripper")
        self.assertEqual(result["HANDBRAKE_HOST"], "qemuhost")
        self.assertEqual(result["BACKUP_HOST"], "mini")
        self.assertEqual(result["HANDBRAKE_ENCODER"], "x265")
        self.assertEqual(result["HANDBRAKE_QUALITY"], "22")
        self.assertIn("openclaw", result["OPENCLAW_BIN"])


if __name__ == "__main__":
    unittest.main()
