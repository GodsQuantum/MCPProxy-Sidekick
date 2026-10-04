#!/lsiopy/bin/python3
import asyncio
import os
import re

LISTEN_HOST = os.environ.get("SIDEKICK_CDP_RELAY_HOST", "0.0.0.0")
LISTEN_PORT = int(os.environ.get("SIDEKICK_CDP_RELAY_PORT", "9223"))
TARGET_HOST = os.environ.get("SIDEKICK_CDP_TARGET_HOST", "127.0.0.1")
TARGET_PORT = int(os.environ.get("SIDEKICK_CDP_TARGET_PORT", "9222"))


def request_host(request_head: bytes) -> str:
    for line in request_head.rstrip(b"\r\n").split(b"\r\n")[1:]:
        if line.lower().startswith(b"host:"):
            return line.split(b":", 1)[1].strip().decode("ascii", "ignore")
    return ""


def is_websocket_upgrade(request_head: bytes) -> bool:
    has_upgrade = False
    connection_upgrade = False
    for line in request_head.rstrip(b"\r\n").split(b"\r\n")[1:]:
        lower = line.lower()
        if lower.startswith(b"upgrade:") and b"websocket" in lower:
            has_upgrade = True
        if lower.startswith(b"connection:") and b"upgrade" in lower:
            connection_upgrade = True
    return has_upgrade and connection_upgrade


def rewrite_host(
    request_head: bytes,
    target_host: str = TARGET_HOST,
    target_port: int = TARGET_PORT,
    preserve_connection: bool = False,
) -> bytes:
    lines = request_head.rstrip(b"\r\n").split(b"\r\n")
    saw_connection = False
    for index, line in enumerate(lines):
        lower = line.lower()
        if lower.startswith(b"host:"):
            lines[index] = f"Host: {target_host}:{target_port}".encode()
        elif lower.startswith(b"connection:"):
            saw_connection = True
            if not preserve_connection:
                lines[index] = b"Connection: close"
    if not saw_connection and not preserve_connection:
        lines.append(b"Connection: close")
    return b"\r\n".join(lines) + b"\r\n\r\n"


def rewrite_websocket_urls(body: bytes, public_host: str) -> bytes:
    if not public_host:
        return body
    replacement = f"ws://{public_host}".encode()
    body = body.replace(f"ws://{TARGET_HOST}:{TARGET_PORT}".encode(), replacement)
    body = body.replace(f"ws://localhost:{TARGET_PORT}".encode(), replacement)
    body = body.replace(f"ws://127.0.0.1:{TARGET_PORT}".encode(), replacement)
    return body


def rewrite_content_length(response_head: bytes, body_length: int) -> bytes:
    lines = response_head.rstrip(b"\r\n").split(b"\r\n")
    replaced = False
    for index, line in enumerate(lines):
        if line.lower().startswith(b"content-length:"):
            lines[index] = f"Content-Length:{body_length}".encode()
            replaced = True
            break
    if not replaced:
        lines.append(f"Content-Length:{body_length}".encode())
    return b"\r\n".join(lines) + b"\r\n\r\n"


def content_length(response_head: bytes):
    match = re.search(br"(?im)^content-length:\s*(\d+)\s*$", response_head)
    return int(match.group(1)) if match else None


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
    upstream_writer = None
    try:
        request_head = await client_reader.readuntil(b"\r\n\r\n")
        public_host = request_host(request_head)
        websocket = is_websocket_upgrade(request_head)
        upstream_reader, upstream_writer = await asyncio.open_connection(
            TARGET_HOST, TARGET_PORT
        )
        upstream_writer.write(
            rewrite_host(request_head, preserve_connection=websocket)
        )
        await upstream_writer.drain()

        if websocket:
            await asyncio.gather(
                pump(client_reader, upstream_writer),
                pump(upstream_reader, client_writer),
                return_exceptions=True,
            )
            return

        response_head = await upstream_reader.readuntil(b"\r\n\r\n")
        length = content_length(response_head)
        if length is None:
            body = await upstream_reader.read()
        else:
            body = await upstream_reader.readexactly(length)

        body = rewrite_websocket_urls(body, public_host)
        response_head = rewrite_content_length(response_head, len(body))
        client_writer.write(response_head + body)
        await client_writer.drain()
    except (OSError, asyncio.IncompleteReadError, asyncio.LimitOverrunError):
        pass
    finally:
        for writer in (upstream_writer, client_writer):
            if writer is None:
                continue
            try:
                writer.close()
                await writer.wait_closed()
            except Exception:
                pass


async def main():
    server = await asyncio.start_server(handle, LISTEN_HOST, LISTEN_PORT)
    async with server:
        await server.serve_forever()


if __name__ == "__main__":
    asyncio.run(main())
