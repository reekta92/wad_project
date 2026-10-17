"""Keep the REST contract and executable request inventory aligned."""

import json
from pathlib import Path
import re
import unittest

ROOT = Path(__file__).resolve().parents[1]


class ContractCoverageTest(unittest.TestCase):
    def test_all_implemented_methods_have_postman_requests(self):
        operations = set()
        path = None
        for line in (ROOT / "docs/openapi.yaml").read_text().splitlines():
            match = re.fullmatch(r"  (/[^:]+):", line)
            if match:
                path = match[1]
            match = re.fullmatch(r"    (get|post|put|delete):", line)
            if match and path:
                operations.add((match[1].upper(), path))
        collection = json.loads((ROOT / "docs/postman/exams.postman_collection.json").read_text())
        requests = set()

        def inspect(items):
            for item in items:
                if "item" in item:
                    inspect(item["item"])
                if "request" in item:
                    request = item["request"]
                    path = request["url"].split("/api/v1", 1)[1].split("?", 1)[0]
                    requests.add((request["method"], re.sub(r"\{\{[^}]+\}\}", "{id}", path)))

        inspect(collection["item"])
        # The unknown-path request deliberately verifies the JSON 404 fallback.
        self.assertEqual(operations, requests - {("GET", "/missing")})
        for resource in ("departments", "courses", "classrooms", "sessions", "personnel", "unavailability-requests", "exams", "invigilator-assignments"):
            for method, suffix in (("POST", ""), ("GET", "/{id}"), ("PUT", "/{id}"), ("DELETE", "/{id}"), ("GET", "")):
                self.assertIn((method, "/" + resource + suffix), operations)


if __name__ == "__main__":
    unittest.main()
