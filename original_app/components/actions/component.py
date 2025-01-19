# DEPENDENCIES
## Built-in
import asyncio
## Third-Party
from nats.aio.client import Client as NatsClient
## Local
from components import broker
from constants.settings import DebugLevels
from helpers import debug_print


async def nats_test(queue: str) -> None:
    nats_client = NatsClient()
    try:
        nats_client, stream = await broker.connect()
        await asyncio.sleep(5)
        for i in range(10):
            await broker.publish(stream=stream, queue=queue, message=f"Hi from {queue}\nMsg: {i}")
        while True:
            pass
    except Exception as error:
        debug_print(f"ConnectionClosedError: {error}...", DebugLevels.ERROR)
    await nats_client.close()

# MAIN
def component(component_type: str) -> None:
    asyncio.run(nats_test(queue=component_type))