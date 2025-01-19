# DEPENDENCIES
## Built-in
import asyncio
from threading import Thread
## Third-Party
from nats.aio.client import Client as NatsClient
from nats.js.client import JetStreamContext
from pydantic import ValidationError
## Local
from components import broker
from components.layer.layer_messages import LayerMessage
from constants.queue import BusKeys, BUSSES_DOWN, BUSSES_UP
from .models import BusMessage


# QUEUE
async def _connect_and_publish(queue: str, layer_message: LayerMessage) -> None:
    nats_client: NatsClient
    nats_stream: JetStreamContext
    nats_client, nats_stream = await broker.connect()
    await broker.publish(stream=nats_stream, queue=queue, message=layer_message.model_dump_json())
    await nats_client.close()

def _bus_publish(bus_direction: str, bus_message: BusMessage) -> None:
    try:
        queue_mapping: dict[str, str] = BUSSES_DOWN if bus_direction == BusKeys.DOWN else BUSSES_UP
        queue: str = queue_mapping.get(bus_message.source_queue, "")
        if not queue:
            print(f"Queue {bus_message.source_queue} cannot send {bus_direction}!")
            return

        layer_message: LayerMessage = bus_message.layer_message
        print(f"Sending {layer_message.message_type}: {layer_message.messages} to {queue}...")

        loop = asyncio.new_event_loop()
        asyncio.set_event_loop(loop)
        thread = Thread(target=loop.run_until_complete, args=(_connect_and_publish(queue, layer_message),))
        thread.start()
        thread.join()
    except ValidationError as error:
        raise error

def bus_down(bus_message: BusMessage) -> None:
    _bus_publish(BusKeys.DOWN, bus_message)

def bus_up(bus_message: BusMessage) -> None:
    _bus_publish(BusKeys.UP, bus_message)
