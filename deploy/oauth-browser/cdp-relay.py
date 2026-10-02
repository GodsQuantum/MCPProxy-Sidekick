#!/lsiopy/bin/python3
import asyncio
import os

LISTEN_HOST = os.environ.get("SIDEKICK_CDP_RELAY_HOST", "0.0.0.0")
LISTEN_PORT = int(os.environ.get("SIDEKICK_CDP_RELAY_PORT", "9223"))
TARGET_HOST = os.environ.get("SIDEKICK_CDP_TARGET_HOST", "127.0.0.1")
TARGET_PORT = int(os.environ.get("SIDEKICK_CDP_TARGET_PORT", "9222"))


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
    except OSError:
        client_writer.close()
        await client_writer.wait_closed()
        return

    await asyncio.gather(
        pump(client_reader, upstream_writer),
        pump(upstream_reader, client_writer),
        return_exceptions=True,
    )


async def main():
    server = await asyncio.start_server(handle, LISTEN_HOST, LISTEN_PORT)
    async with server:
        await server.serve_forever()


asyncio.run(main())
