# DEPENDENCIES
## Built-in
from typing import Awaitable, Callable
## Third-Party
import nats
from nats.aio.client import Client as NatsClient
from nats.js.client import JetStreamContext
from nats.aio.msg import Msg as NatsMsg
from nats.errors import TimeoutError
from tenacity import retry, wait_exponential, retry_if_exception_type
## Local
from constants.settings import DebugLevels
from helpers import debug_print


@retry(retry=retry_if_exception_type(ConnectionRefusedError), wait=wait_exponential(multiplier=1, max=10))
async def connect(ip_address: str = "127.0.0.1") -> tuple[NatsClient, JetStreamContext]:
    print(f"Connecting to NATS Client on {ip_address}...")
    nats_client: NatsClient = await nats.connect(f"nats://{ip_address}:4222")
    stream: JetStreamContext = nats_client.jetstream()
    print("Connected Successfully!")
    return nats_client, stream

async def establish_queues(stream: JetStreamContext, queues: frozenset[str]) -> None:
    print("Establishing queues...")
    for queue in queues:
        try:
            print(f"Trying to delete stream {queue}...")
            await stream.delete_stream(name=queue)
        except Exception:
            pass

        print(f"Creating stream {queue}...")
        await stream.add_stream(name=queue, subjects=[queue])
        debug_print(f"Established queue: {queue}...")
    print("Established all queues!")

async def subscribe(stream: JetStreamContext, handler: Callable[[NatsMsg], Awaitable[None]], queue: str) -> None:
    print(f"Subscribing to {queue}...")
    sub = await stream.subscribe(queue, durable=queue)
    print(f"Subscribed to {queue}...")
    while True:
        try:
            message: NatsMsg = await sub.next_msg()
            await handler(message)
        except TimeoutError:
            continue
        except ConnectionRefusedError:
            _, stream = await connect()

async def publish(stream: JetStreamContext, queue: str, message: str) -> None:
    print(f"Publishing {message} to {queue}...", DebugLevels.DEBUG)
    ack = await stream.publish(queue, message.encode())

async def request(nats_client: NatsClient, queue: str, message: str, timeout: float = 30) -> None:
    print(f"Requesting {message} from {queue}...", DebugLevels.DEBUG)
    try:
        response = await nats_client.request(queue, message.encode(), timeout=timeout)
        print(f"Received response: {response.data.decode()}")
    except TimeoutError:
        print(f"Request to {queue} timed out")
