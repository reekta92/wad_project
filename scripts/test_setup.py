import importlib.util
from pathlib import Path
import stat
import tempfile
import unittest

spec = importlib.util.spec_from_file_location("setup", Path(__file__).with_name("setup.py"))
setup = importlib.util.module_from_spec(spec)
spec.loader.exec_module(setup)


class SetupTest(unittest.TestCase):
    def test_credentials_are_private_distinct_and_never_overwritten(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / ".env.example").write_text("\n".join(f"{key}=" for key in setup.SECRET_NAMES))
            self.assertTrue(setup.create_environment(root))
            content = (root / ".env").read_text()
            values = [line.split("=", 1)[1] for line in content.splitlines()]
            self.assertEqual(len(set(values)), len(setup.SECRET_NAMES))
            self.assertTrue(all(len(value) >= 32 for value in values))
            self.assertEqual(stat.S_IMODE((root / ".env").stat().st_mode), 0o600)
            self.assertFalse(setup.create_environment(root))
            self.assertEqual(content, (root / ".env").read_text())


if __name__ == "__main__":
    unittest.main()
