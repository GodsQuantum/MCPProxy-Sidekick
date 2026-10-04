#!/usr/bin/env python3
import importlib.util
from pathlib import Path
import unittest

MODULE_PATH = Path(__file__).with_name("cdp-relay.py")
SPEC = importlib.util.spec_from_file_location("cdp_relay", MODULE_PATH)
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


class CDPRelayRewriteTests(unittest.TestCase):
    def test_rewrites_docker_hostname_for_chromium(self):
        request = (
            b"GET /json/version HTTP/1.1\r\n"
            b"Host: playwright-arezki-browser:9223\r\n"
            b"User-Agent: test\r\n\r\n"
        )
        result = MODULE.rewrite_host(request)
        self.assertIn(b"Host: 127.0.0.1:9222\r\n", result)
        self.assertIn(b"Connection: close\r\n", result)
        self.assertNotIn(b"Host: playwright-arezki-browser:9223", result)

    def test_host_match_is_case_insensitive(self):
        request = b"GET / HTTP/1.1\r\nhOsT: browser:9223\r\n\r\n"
        result = MODULE.rewrite_host(request, "localhost", 9222)
        self.assertIn(b"Host: localhost:9222\r\n", result)

    def test_extracts_original_host(self):
        request = b"GET / HTTP/1.1\r\nHost: playwright-fella-browser:9223\r\n\r\n"
        self.assertEqual(
            MODULE.request_host(request), "playwright-fella-browser:9223"
        )

    def test_preserves_websocket_upgrade(self):
        request = (
            b"GET /devtools/browser/id HTTP/1.1\r\n"
            b"Host: playwright-fella-browser:9223\r\n"
            b"Connection: Upgrade\r\n"
            b"Upgrade: websocket\r\n\r\n"
        )
        self.assertTrue(MODULE.is_websocket_upgrade(request))
        result = MODULE.rewrite_host(request, preserve_connection=True)
        self.assertIn(b"Connection: Upgrade\r\n", result)
        self.assertNotIn(b"Connection: close", result)

    def test_rewrites_cdp_websocket_url_for_external_client(self):
        body = (
            b'{"webSocketDebuggerUrl":'
            b'"ws://127.0.0.1:9222/devtools/browser/abc"}'
        )
        result = MODULE.rewrite_websocket_urls(
            body, "playwright-fella-browser:9223"
        )
        self.assertIn(
            b"ws://playwright-fella-browser:9223/devtools/browser/abc", result
        )
        self.assertNotIn(b"ws://127.0.0.1:9222", result)

    def test_updates_content_length(self):
        head = b"HTTP/1.1 200 OK\r\nContent-Length:402\r\n\r\n"
        result = MODULE.rewrite_content_length(head, 433)
        self.assertIn(b"Content-Length:433\r\n", result)


if __name__ == "__main__":
    unittest.main()
