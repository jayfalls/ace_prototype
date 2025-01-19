# DEPENDENCIES
## Built-in
import asyncio
from concurrent import futures
## Local
from constants.queue import QueueCommands, QUEUES
from helpers import execute
from . import broker


async def queue_server() -> None:
    print("Starting queue server...")
    loop = asyncio.get_running_loop()
    executor = futures.ThreadPoolExecutor()
    task = loop.run_in_executor(executor, execute, QueueCommands.START)
    print("Establishing queues...")
    _, stream = await broker.connect()
    await broker.establish_queues(stream=stream, queues=QUEUES)
    await task

# MAIN
def component(component_type: str) -> None:
    asyncio.run(queue_server())