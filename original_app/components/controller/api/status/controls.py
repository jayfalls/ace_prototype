# DEPENDENCIES
## Built-in
from typing import Union
## Local
from components.layer.layer_messages import LayerMessage
from constants.layer import LayerKeys, LayerCommands
from constants.queue import Queues
from ..bus.service import bus_down
from ..bus.models import BusMessage


# PRIVATE
def _power_on_self_test() -> None:
    commands_dict: dict[str, Union[str, dict[str, str]]] = {
        LayerKeys.MESSAGE_TYPE: LayerKeys.COMMANDS, 
        LayerKeys.MESSAGES: {
            LayerKeys.HEADING: LayerKeys.ACTIONS, 
            LayerKeys.CONTENT: LayerCommands.POST
        }
    }
    commands_message: LayerMessage = LayerMessage.model_validate(commands_dict)
    post_payload = BusMessage(source_queue=Queues.CONTROLLER, layer_message=commands_message)
    try:
        bus_down(post_payload)
    except Exception as error:
        raise error


# PUBLIC
def start_ace() -> None:
    try:
        _power_on_self_test()
    except Exception as error:
        raise error
