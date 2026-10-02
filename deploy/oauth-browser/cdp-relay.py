#!/lsiopy/bin/python3
import asyncio
import os

LISTEN_HOST = os.environ.get("SIDEKICK_CDP_RELAY_HOST", "0.0.0.0")
LISTEN_PORT = int(os.environ.get("SIDEKICK_CDP_RELAY_PORT", "9223"))
TARGET_HOST = os.environ.get("SIDEKICK_CDP_TARGET_HOST", "127.0.0.1")
TARGET_PORT = int(os.environ.get("SIDEKICK_CDP_TARGET_PORT", "9222"))


def rewrite_host(request_head, target_host=TARGET_HOST, target_port=TARGET_PORT):
    # Chromium rejects non-local Host values on the DevTools HTTP endpoint.
    # The relay handles one request per TCP connection, so also force the
    # upstream response to close the connection. This prevents HTTP clients
    # from reusing the socket for a second request whose Host would bypass
    # this first-request rewrite.
    lines = request_head.rstrip(b"\r\n").split(b"\r\n")
    saw_connection = False
    for index, line in enumerate(lines):
        lower = line.lower()
        if lower.startswith(b"host:"):
            lines[index] = f"Host: {target_host}:{target_port}".encode()
        elif lower.startswith(b"connection:"):
            lines[index] = b"Connection: close"
            saw_connection = True
    if not saw_connection:
        lines.append(b"Connection: close")
    return b"\r\n".join(lines) + b"\r\n\r\n"


async def pump(reader, writer):
    try:
        while True:
            data = await reader.read(65536)
            if not data:
                break
            writer.write(data)
            await writer.drain()
    except (ConnectionError, asyncio.CancelledError):
        pass
    finally:
        try:
            writer.close()
            await writer.wait_closed()
        except Exception:
            pass


async def handle(client_reader, client_writer):
    try:
        upstream_reader, upstream_writer = await asyncio.open_connection(
            TARGET_HOST, TARGET_PORT
        )
        request_head = await client_reader.readuntil(b"\r\n\r\n")
    except (OSError, asyncio.IncompleteReadError, asyncio.LimitOverrunError):
        client_writer.close()
        await client_writer.wait_closed()
        return

    upstream_writer.write(rewrite_host(request_head))
    await upstream_writer.drain()

    await asyncio.gather(
        pump(client_reader, upstream_writer),
        pump(upstream_reader, client_writer),
        return_exceptions=True,
    )


async def main():
    server = await asyncio.start_server(handle, LISTEN_HOST, LISTEN_PORT)
    async with server:
        await server.serve_forever()


if __name__ == "__main__":
    asyncio.run(main())
