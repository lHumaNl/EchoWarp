import logging
import os

from logging_config import setup_logging
from settings import Settings

setup_logging()


def get_settings_interactive() -> Settings:
    address = None
    mode = None

    while mode not in [Settings.get_client_value()[0], Settings.get_client_value()[1]]:
        if mode is not None:
            logging.error(f'Selected invalid id of util mode: {mode}')

        try:
            mode = input(
                f"Select id of util mode:{os.linesep}"
                f"{Settings.get_client_value()[0]}. {Settings.get_client_value()[1]}{os.linesep}"
                f"{Settings.get_server_value()[0]}. {Settings.get_server_value()[1]}{os.linesep}")

            mode = int(mode)
        except Exception:
            pass

    for value in [Settings.get_client_value(), Settings.get_server_value()]:
        if mode == value[0]:
            logging.info(f"Selected mode: {value[1]}")

    if mode == Settings.get_client_value()[0]:
        is_server = False
    else:
        is_server = True

    try:
        port = int(input(f"Select port (default={Settings.get_default_port()}):"))
    except Exception:
        port = Settings.get_default_port()

    logging.info(f"Selected port: {port}")

    if mode == 1:
        address = input(f"Select server host:{os.linesep}")
        logging.info(f"Selected server host: {address}")

    return Settings(is_server, port, address)
