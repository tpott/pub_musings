"""Tests for case-insensitive state file lookup in state.resolve_state_arg."""

import json
import shutil
import tempfile
import unittest
from pathlib import Path

from state import new_state, resolve_state_arg, save_state


class TestResolveStateArgCasing(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.state_dir = Path(self.tmpdir) / ".state"
        self.state_dir.mkdir(parents=True)

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def _write_state_file(self, label):
        """Write a minimal state file for the given label."""
        path = self.state_dir / f"{label}.json"
        state = new_state(self.tmpdir, label)
        state["_state_path"] = str(path)
        with open(path, "w") as f:
            json.dump(state, f)
        return path

    def test_resolve_state_arg_finds_case_variant(self):
        """resolve_state_arg finds state file even when label casing differs."""
        self._write_state_file("Legend_Of_Korra_Book_3_Disc_1")
        state = resolve_state_arg(self.tmpdir, "Legend_of_Korra_Book_3_Disc_1")
        self.assertEqual(state["disc_label"], "Legend_Of_Korra_Book_3_Disc_1")

    def test_resolve_state_arg_exact_match_unchanged(self):
        """Exact-case lookup still works after adding case-insensitive fallback."""
        self._write_state_file("SHE_RA_S1_D1")
        state = resolve_state_arg(self.tmpdir, "SHE_RA_S1_D1")
        self.assertEqual(state["disc_label"], "SHE_RA_S1_D1")

    def test_resolve_state_arg_ambiguous_raises(self):
        """Two state files with same lowercase stem should raise with both listed.

        Uses a third casing as the lookup key so neither file is an exact match,
        forcing the case-insensitive fallback path where ambiguity is detected.
        """
        self._write_state_file("Legend_Of_Korra")
        self._write_state_file("legend_of_korra")
        with self.assertRaises(RuntimeError) as ctx:
            resolve_state_arg(self.tmpdir, "LEGEND_OF_KORRA")
        msg = str(ctx.exception)
        self.assertIn("Legend_Of_Korra", msg)
        self.assertIn("legend_of_korra", msg)

    def test_resolve_state_arg_missing_still_raises(self):
        """Nonexistent label raises RuntimeError as before."""
        with self.assertRaises(RuntimeError):
            resolve_state_arg(self.tmpdir, "NONEXISTENT_DISC")


if __name__ == "__main__":
    unittest.main()
