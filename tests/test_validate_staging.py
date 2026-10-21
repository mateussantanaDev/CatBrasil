"""Release gate must reject stale schema and unhealthy staging containers."""

import json
import unittest
from unittest.mock import patch

from scripts.release import validate_staging


class ValidateStagingTests(unittest.TestCase):
    def test_source_migrations_are_contiguous(self):
        versions = validate_staging.migration_versions()
        self.assertEqual(versions, list(range(1, versions[-1] + 1)))
        self.assertGreaterEqual(versions[-1], 14)

    def test_unhealthy_api_blocks_gate(self):
        services = {name: {"State": "running", "Health": "healthy", "ID": name}
                    for name in validate_staging.RUNNING}
        services["backend-migrate"] = {"State": "exited", "ExitCode": 0}
        self.assertEqual(validate_staging.check_services(services), [])
        services["backend-api"]["Health"] = "unhealthy"
        self.assertIn("backend-api: healthcheck não está healthy",
                      validate_staging.check_services(services))

    def test_stale_schema_blocks_gate(self):
        services = {name: {"State": "running", "Health": "healthy", "ID": name}
                    for name in validate_staging.RUNNING}
        services["backend-migrate"] = {"State": "exited", "ExitCode": 0}
        ps = "\n".join(json.dumps({"Service": name, **entry})
                       for name, entry in services.items())
        with patch.object(validate_staging, "run", side_effect=[ps, "1,2,3,4,5,6,7,8\n"]), \
             patch.object(validate_staging.Path, "is_file", return_value=True):
            self.assertEqual(validate_staging.main([]), 1)


if __name__ == "__main__":
    unittest.main()
