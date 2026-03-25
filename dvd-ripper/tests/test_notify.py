"""Tests for rip.notify — verifies shell-safe quoting of messages."""

import unittest
from unittest.mock import patch, call

from rip import notify


class TestNotify(unittest.TestCase):

    @patch("rip.run")
    def test_message_with_single_quotes(self, mock_run):
        """Messages containing single quotes must not break the shell command."""
        conf = {
            "OPENCLAW_BIN": "/usr/bin/openclaw",
            "OPENCLAW_TARGET": "@user:matrix.org",
        }
        # Simulate the CalledProcessError message that contains single quotes
        message = (
            "Rip FAILED: Command 'ssh mbp4 'HandBrakeCLI"
            " -i /work/file.mkv -o /work/out.mp4"
            " -e x265 --encoder-preset medium -q 22 -B 160''"
            " returned non-zero exit status 2."
        )
        notify(conf, message)

        # The command passed to run() must be a single valid shell string.
        # Verify run() was called exactly once and the message is properly quoted
        # (not split into multiple shell tokens).
        mock_run.assert_called_once()
        cmd = mock_run.call_args[0][0]
        self.assertIn("--message", cmd)
        # The raw single quotes from the message must NOT appear unescaped
        # between the outer single quotes. shlex.quote wraps in single quotes
        # and escapes internal single quotes.
        self.assertNotIn("--message '" + message + "'", cmd)

    @patch("rip.run")
    def test_simple_message(self, mock_run):
        """A simple message without special characters still works."""
        conf = {
            "OPENCLAW_BIN": "/usr/bin/openclaw",
            "OPENCLAW_TARGET": "@user:matrix.org",
        }
        notify(conf, "Rip complete: Movie is ready in Jellyfin")
        mock_run.assert_called_once()
        cmd = mock_run.call_args[0][0]
        self.assertIn("Rip complete: Movie is ready in Jellyfin", cmd)


if __name__ == "__main__":
    unittest.main()
