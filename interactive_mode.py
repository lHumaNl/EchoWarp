import os

from settings import Settings
from utils import select_in_interactive


def get_settings_interactive() -> Settings:
    address = None

    is_server_mode = select_in_interactive('util mode', Settings.get_util_mods_list())
    is_input_device = select_in_interactive('audio device', Settings.get_device_list())

    try:
        port = int(input(f"Select port (default={Settings.get_default_port()}):"))
    except Exception:
        port = Settings.get_default_port()

    if not is_server_mode:
        address = input(f"Select server host:{os.linesep}")

    encoding = input(f"Select device names encoding (default={Settings.get_default_encoding()}):")
    if encoding == '':
        encoding = Settings.get_default_encoding()

    return Settings(is_server_mode, is_input_device, port, address, encoding)
