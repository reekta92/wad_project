import importlib.util
from pathlib import Path
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("https_demo", Path(__file__).with_name("prepare_https_demo.py"))
demo = importlib.util.module_from_spec(spec)
spec.loader.exec_module(demo)


class HTTPSGuardTest(unittest.TestCase):
    def test_local_workspace_is_not_reconfigured(self):
        with patch.dict("os.environ", {"CI": "false"}), patch.object(demo.subprocess, "run") as command:
            with self.assertRaisesRegex(ValueError, "isolated CI"):
                demo.prepare()
            command.assert_not_called()


if __name__ == "__main__":
    unittest.main()
