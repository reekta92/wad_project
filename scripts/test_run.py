import importlib.util
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location("runner", Path(__file__).with_name("run.py"))
runner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runner)


class EnvironmentTest(unittest.TestCase):
    def test_values_are_literal_and_explicit_environment_wins(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / ".env").write_text("# local\nTOKEN=$(must-not-execute)\nMODE=local\n")
            result = runner.environment(root, {"MODE": "ci"})
            self.assertEqual(result, {"TOKEN": "$(must-not-execute)", "MODE": "ci"})

    def test_invalid_assignment_is_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / ".env").write_text("export SECRET=unexpected\n")
            with self.assertRaises(ValueError):
                runner.environment(root, {})


if __name__ == "__main__":
    unittest.main()
