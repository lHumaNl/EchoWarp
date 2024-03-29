import logging
import os
from typing import Optional, List

from crypto_manager import CryptoManager
from settings.audio_device import AudioDevice
from logging_config import setup_logging

setup_logging()


class Settings:
    is_server: bool
    is_input_device: bool
    udp_port: int
    server_addr: Optional[str]
    encoding: str
    heartbeat_attempt: int
    audio_device: AudioDevice
    is_ssl: bool
    is_hash_control: bool
    crypto_manager: Optional[CryptoManager]

    __CLIENT_MODE = [1, 'client']
    __SERVER_MODE = [2, 'server']
    __MODS_LIST = [__CLIENT_MODE, __SERVER_MODE]

    __OUTPUT_DEVICE = [1, 'output']
    __INPUT_DEVICE = [2, 'input']
    __DEVICE_LIST = [__OUTPUT_DEVICE, __INPUT_DEVICE]

    __SSL_ENABLE = [1, 'True']
    __SSL_DISABLE = [2, 'False']
    __SSL_LIST = [__SSL_ENABLE, __SSL_DISABLE]

    __HASH_CONTROL_ENABLE = [1, 'True']
    __HASH_CONTROL_DISABLE = [2, 'False']
    __HASH_CONTROL_LIST = [__HASH_CONTROL_ENABLE, __HASH_CONTROL_DISABLE]

    __DEFAULT_PORT = 6532
    __DEFAULT_ENCODING = 'cp1251'
    __DEFAULT_HEARTBEAT_ATTEMPT = 5

    def __init__(self, is_server_mode: bool, is_input_device: bool, udp_port: int, server_addr: Optional[str],
                 encoding: str, heartbeat_attempt: int, is_ssl: bool, is_hash_control: bool):
        self.is_server = is_server_mode
        self.is_input_device = is_input_device
        self.udp_port = udp_port
        self.server_addr = server_addr
        self.encoding = encoding
        self.heartbeat_attempt = heartbeat_attempt
        self.audio_device = AudioDevice(self.is_input_device, self.encoding)
        self.is_ssl = is_ssl
        self.is_hash_control = is_hash_control
        self.crypto_manager = CryptoManager(self.is_server, self.is_hash_control)

        self.__print_setting()

    def __print_setting(self):
        if self.is_server:
            settings_print_string = f'Selected util mode: {self.__SERVER_MODE[1] + os.linesep}'

        else:
            settings_print_string = f'Selected util mode: {self.__CLIENT_MODE[1] + os.linesep}'

        if self.is_input_device:
            settings_print_string += f'Selected audio device type: {self.__INPUT_DEVICE[1] + os.linesep}'
        else:
            settings_print_string += f'Selected audio device type: {self.__OUTPUT_DEVICE[1] + os.linesep}'

        settings_print_string += f'Selected port: {self.udp_port}{os.linesep}'

        if self.server_addr is not None:
            settings_print_string += f'Selected server host: {self.server_addr}{os.linesep}'

        settings_print_string += f'Selected encoding: {self.encoding + os.linesep}'
        settings_print_string += f'Selected heartbeat attempt: {self.heartbeat_attempt}{os.linesep}'
        settings_print_string += f'Selected ssl mode: {self.is_ssl}{os.linesep}'
        settings_print_string += f'Selected integrity control: {self.is_hash_control}{os.linesep}'
        settings_print_string += (f'Selected audio device: {self.audio_device.device_name}, '
                                  f'Channels: {self.audio_device.channels}, '
                                  f'Sample rate: {self.audio_device.sample_rate}Hz{os.linesep}')

        logging.info(f"Start util with settings:{os.linesep + settings_print_string}")

    @staticmethod
    def get_util_mods_list() -> List:
        return Settings.__MODS_LIST

    @staticmethod
    def get_device_list() -> List:
        return Settings.__DEVICE_LIST

    @staticmethod
    def get_ssl_list() -> List:
        return Settings.__SSL_LIST

    @staticmethod
    def get_hash_control_list() -> List:
        return Settings.__HASH_CONTROL_LIST

    @staticmethod
    def get_default_port():
        return Settings.__DEFAULT_PORT

    @staticmethod
    def get_default_encoding():
        return Settings.__DEFAULT_ENCODING

    @staticmethod
    def get_default_heartbeat_attempt():
        return Settings.__DEFAULT_HEARTBEAT_ATTEMPT
