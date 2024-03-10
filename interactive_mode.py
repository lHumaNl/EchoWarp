import logging
import os
from typing import List

from settings import Settings


def get_settings_interactive() -> Settings:
    address = None

    is_server_mode = select_in_interactive_from_values('util mode', Settings.get_util_mods_list())
    is_input_device = select_in_interactive_from_values('audio device', Settings.get_device_list())

    port = input_in_interactive_int_value(Settings.get_default_port(), 'port')

    if not is_server_mode:
        address = input(f"Select server host:{os.linesep}")

    encoding = input(f"Select device names encoding (default={Settings.get_default_encoding()}):")
    if encoding == '':
        encoding = Settings.get_default_encoding()

    heartbeat_attempt = input_in_interactive_int_value(Settings.get_default_heartbeat_attempt(), 'heartbeat attempt')

    return Settings(is_server_mode, is_input_device, port, address, encoding, heartbeat_attempt)


def input_in_interactive_int_value(default_value: int, descr: str) -> int:
    value = None

    while not type(value) is int:
        try:
            value = input(f"Select {descr} (default={default_value}):")

            if value == '':
                value = default_value
                break

            value = int(value)
        except Exception:
            pass

    return value


def select_in_interactive_from_values(descr: str, list_values: List):
    mode = None

    while mode not in [val[0] for val in list_values]:
        if mode is not None:
            logging.error(f'Selected invalid id of {descr}: {mode}')

        try:
            select_str = f"Select id of {descr}:{os.linesep}"
            values_str = ''

            for value in list_values:
                values_str += f"{value[0]}. {value[1]}{os.linesep}"

            mode = input(select_str + values_str)
            mode = int(mode)
        except Exception:
            pass

    return mode == list_values[0][0]
