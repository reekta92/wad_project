import importlib.util
from pathlib import Path
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("sonar_ci", Path(__file__).with_name("sonar_ci.py"))
sonar = importlib.util.module_from_spec(spec)
spec.loader.exec_module(sonar)


class SonarGuardTest(unittest.TestCase):
    def test_disposable_server_cannot_run_in_a_normal_workspace(self):
        with patch.dict("os.environ", {"CI": "false"}), patch.object(sonar.subprocess, "run") as command:
            with self.assertRaisesRegex(ValueError, "isolated CI"):
                sonar.run()
            command.assert_not_called()

    def test_overall_defects_fail_even_when_first_analysis_gate_passes(self):
        report = {"quality_gate": {"projectStatus": {"status": "OK"}}, "measures": {"component": {"measures": [{"metric": key, "value": "0"} for key in ("bugs", "vulnerabilities", "security_hotspots")]}}}
        sonar.verify_report(report)
        for metric in report["measures"]["component"]["measures"]:
            metric["value"] = "1"
            with self.assertRaisesRegex(RuntimeError, "defects"):
                sonar.verify_report(report)
            metric["value"] = "0"
        report["quality_gate"]["projectStatus"]["status"] = "ERROR"
        with self.assertRaisesRegex(RuntimeError, "quality gate"):
            sonar.verify_report(report)


if __name__ == "__main__":
    unittest.main()
