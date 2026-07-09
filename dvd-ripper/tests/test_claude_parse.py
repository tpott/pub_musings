"""Tests for claude._extract_json — robust JSON extraction from model output.

The verify pipeline saw the model wrap its JSON verdict in a prose preamble
("Now I have all the information needed. Let me compile the final JSON
response. ...{json}"), which the old parser (leading-fence strip + json.loads)
rejected, silently discarding all the structured verification work. These
tests pin down the lenient extraction behavior that prevents that.
"""

import json
import unittest

import claude


CLEAN = {"verdict": "pass", "confidence": 0.9, "issues": []}


class ExtractJsonTest(unittest.TestCase):
    def test_clean_json(self):
        self.assertEqual(claude._extract_json(json.dumps(CLEAN)), CLEAN)

    def test_fenced_json(self):
        text = "```json\n" + json.dumps(CLEAN) + "\n```"
        self.assertEqual(claude._extract_json(text), CLEAN)

    def test_bare_fence(self):
        text = "```\n" + json.dumps(CLEAN) + "\n```"
        self.assertEqual(claude._extract_json(text), CLEAN)

    def test_prose_preamble_then_json(self):
        # The exact failure mode from the Brooklyn Nine-Nine rip.
        text = (
            "Now I have all the information needed. Let me compile the final "
            "JSON response.\n\n" + json.dumps(CLEAN)
        )
        self.assertEqual(claude._extract_json(text), CLEAN)

    def test_json_then_trailing_prose(self):
        text = json.dumps(CLEAN) + "\n\nHope that helps!"
        self.assertEqual(claude._extract_json(text), CLEAN)

    def test_prose_with_small_brace_and_real_json(self):
        # A stray brace in prose must not win over the real (larger) object.
        text = (
            "Consider the set {a, b}. Here is the verdict:\n"
            + json.dumps(CLEAN)
        )
        self.assertEqual(claude._extract_json(text), CLEAN)

    def test_nested_object_with_braces_in_strings(self):
        obj = {
            "verdict": "fail",
            "recommendation": "use {curly} braces carefully",
            "title_frame_check": {"attempted": True, "results": []},
        }
        text = "Here you go:\n" + json.dumps(obj) + "\nDone."
        self.assertEqual(claude._extract_json(text), obj)

    def test_unparseable_returns_none(self):
        self.assertIsNone(claude._extract_json("no json here at all"))

    def test_empty_returns_none(self):
        self.assertIsNone(claude._extract_json(""))


if __name__ == "__main__":
    unittest.main()
