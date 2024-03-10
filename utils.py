import logging
import os
from typing import List

from logging_config import setup_logging

setup_logging()


def select_in_interactive(descr: str, list_values: List):
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


def decode_string(string: str, encoding: str) -> str:
    try:
        device_name = string.encode(encoding).decode('utf-8')
    except UnicodeEncodeError:
        device_name = string

    return device_name
