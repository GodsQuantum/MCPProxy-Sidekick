import os
import runpy
import sys
import unittest
from pathlib import Path
from unittest.mock import patch

WRAPPER = str(Path(__file__).with_name("browser-cloak-agent-mcp-wrapper.py"))

class TestCloakAgentBrowserNativeCDP(unittest.TestCase):
    def test_child_mcp_inherits_cdp_and_identity(self):
        scope = runpy.run_path(WRAPPER)
        main = scope["main"]
        main.__globals__["ensure_profile"] = lambda *a: "ws://example.invalid/devtools/browser/mock"
        main.__globals__["AGENT_BROWSER"] = "/bin/true"
        passed = {}
        def capture(path, argv, env):
            passed.update(path=path, argv=argv, env=env)
        with patch.object(os.path, "isfile", return_value=True), patch.object(
            os, "execvpe", side_effect=capture
        ), patch.object(
            sys, "argv",
            ["wrapper", "--profile-id", "mock", "--session", "cloak-fella", "--namespace", "fella"],
        ):
            self.assertEqual(main(), 0)
        self.assertEqual(passed["env"]["AGENT_BROWSER_CDP"], "ws://example.invalid/devtools/browser/mock")
        self.assertEqual(passed["env"]["AGENT_BROWSER_SESSION"], "cloak-fella")
        self.assertEqual(passed["env"]["AGENT_BROWSER_NAMESPACE"], "fella")
        self.assertEqual(passed["argv"][-3:], ["mcp", "--tools", "core,network,state,tabs"])

if __name__ == "__main__":
    unittest.main()
