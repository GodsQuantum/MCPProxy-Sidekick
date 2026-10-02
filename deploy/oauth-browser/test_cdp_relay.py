#!/usr/bin/env python3
import importlib.util
from pathlib import Path
import unittest

MODULE_PATH = Path(__file__).with_name("cdp-relay.py")
SPEC = importlib.util.spec_from_file_location("cdp_relay", MODULE_PATH)
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


class RewriteHostTests(unittest.TestCase):
    def test_rewrites_docker_hostname_for_chromium(self):
        request = (
            b"GET /json/version HTTP/1.1\r\n"
            b"Host: oauth-browser:9223\r\n"
            b"User-Agent: test\r\n\r\n"
        )
        result = MODULE.rewrite_host(request)
        self.assertIn(b"Host: 127.0.0.1:9222\r\n", result)
        self.assertIn(b"Connection: close\r\n", result)
        self.assertNotIn(b"Host: oauth-browser:9223", result)

    def test_host_match_is_case_insensitive(self):
        request = b"GET / HTTP/1.1\r\nhOsT: oauth-browser:9223\r\n\r\n"
        result = MODULE.rewrite_host(request, "localhost", 9222)
        self.assertIn(b"Host: localhost:9222\r\n", result)


if __name__ == "__main__":
    unittest.main()
