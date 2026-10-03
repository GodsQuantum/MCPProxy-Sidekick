#!/usr/bin/env python3
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "render-browser-policy.py"
EXTENSION_ID = "nngceckbapebfimnlniiiahkandclblb"
UPDATE_URL = "https://clients2.google.com/service/update2/crx"

def render(mode, base_url=""):
    with tempfile.TemporaryDirectory() as td:
        out = Path(td) / "bitwarden.json"
        cmd = [sys.executable, str(SCRIPT), "--mode", mode, "--output", str(out)]
        if base_url:
            cmd += ["--base-url", base_url]
        result = subprocess.run(cmd, cwd=ROOT, text=True, capture_output=True)
        if result.returncode != 0:
            return result, None
        return result, json.loads(out.read_text())

class BrowserPolicyTests(unittest.TestCase):
    def test_managed_extension_uses_official_web_store_identity(self):
        result, policy = render("managed-extension")
        self.assertEqual(result.returncode, 0, result.stderr)
        settings = policy["ExtensionSettings"][EXTENSION_ID]
        self.assertEqual(settings["installation_mode"], "force_installed")
        self.assertEqual(settings["update_url"], UPDATE_URL)

    def test_assist_never_force_installs(self):
        result, policy = render("assist", "https://vault.example.com")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertNotIn("ExtensionSettings", policy)
        env = policy["3rdparty"]["extensions"][EXTENSION_ID]["environment"]
        self.assertEqual(env, {"base": "https://vault.example.com"})

    def test_off_is_empty_even_with_base_url(self):
        result, policy = render("off", "https://vault.example.com")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(policy, {})

    def test_managed_extension_can_preconfigure_self_hosted_base(self):
        result, policy = render("managed-extension", "https://vault.example.com/")
        self.assertEqual(result.returncode, 0, result.stderr)
        env = policy["3rdparty"]["extensions"][EXTENSION_ID]["environment"]
        self.assertEqual(env, {"base": "https://vault.example.com"})

    def test_rejects_insecure_or_credentialed_base_url(self):
        for value in (
            "http://vault.example.com",
            "https://u:p@localhost",
            "https://vault.example.com/path?token=secret",
            "file:///tmp/vault",
        ):
            with self.subTest(value=value):
                result, policy = render("managed-extension", value)
                self.assertNotEqual(result.returncode, 0)
                self.assertIsNone(policy)

    def test_rejects_unknown_mode(self):
        result, policy = render("magic")
        self.assertNotEqual(result.returncode, 0)
        self.assertIsNone(policy)

if __name__ == "__main__":
    unittest.main()
