import json
import locale
import logging
import os
import sys
import time
from typing import Optional, List, Dict, get_type_hints
import configparser

from echowarp.logging_config import Logger
from echowarp.models.default_values_and_options import DefaultValuesAndOptions
from echowarp.settings import Settings


class ConfigParser:
    is_server: bool
    is_input_audio_device: bool
    udp_port: int
    socket_buffer_size: int
    device_id: Optional[int]
    password: Optional[str]
    device_encoding_names: str
    ignore_device_encoding_names: bool
    is_error_log: bool
    server_address: Optional[str]
    reconnect_attempt: int
    is_ssl: bool
    is_integrity_control: bool
    workers: int

    __IS_SERVER = ['is_server',
                   DefaultValuesAndOptions.get_util_mods_options_data().default_value]
    __IS_INPUT_DEVICE = ['is_input_audio_device',
                         DefaultValuesAndOptions.get_audio_device_type_options_data().default_value]
    __UDP_PORT = ['udp_port',
                  DefaultValuesAndOptions.get_default_port()]
    __SOCKET_BUFFER_SIZE = ['socket_buffer_size',
                            DefaultValuesAndOptions.get_socket_buffer_size_options_data().default_value]
    __DEVICE_ID = ['device_id',
                   None]
    __PASSWORD = ['password',
                  None]
    __DEVICE_ENCODING_NAMES = ['device_encoding_names',
                               DefaultValuesAndOptions.get_encoding_charset_options_data().default_value]
    __IGNORE_DEVICE_ENCODING_NAMES = ['ignore_device_encoding_names',
                                      DefaultValuesAndOptions.get_ignore_encoding_options_data().default_value]
    __IS_ERROR_LOG = ['is_error_log',
                      DefaultValuesAndOptions.get_error_log_options_data().default_value]

    __SERVER_ADDRESS = ['server_address',
                        None]
    __RECONNECT_ATTEMPT = ['reconnect_attempt',
                           DefaultValuesAndOptions.get_default_reconnect_attempt()]

    __IS_SSL = ['is_ssl',
                DefaultValuesAndOptions.get_ssl_options_data().default_value]
    __IS_INTEGRITY_CONTROL = ['is_integrity_control',
                              DefaultValuesAndOptions.get_hash_control_options_data().default_value]
    __WORKERS = ['workers',
                 DefaultValuesAndOptions.get_default_workers()]

    __CLIENT_PROPS = [
        __SERVER_ADDRESS,
    ]

    __SERVER_PROPS = [
        __IS_SSL,
        __IS_INTEGRITY_CONTROL,
        __WORKERS,
    ]

    __OTHER_PROPS = [
        __SOCKET_BUFFER_SIZE,
        __RECONNECT_ATTEMPT,
        __IS_SERVER,
        __IS_INPUT_DEVICE,
        __UDP_PORT,
        __DEVICE_ID,
        __PASSWORD,
        __DEVICE_ENCODING_NAMES,
        __IGNORE_DEVICE_ENCODING_NAMES,
        __IS_ERROR_LOG,
    ]

    def __init__(self, filename: str, settings: Settings = None):
        if settings is not None:
            self.__save_config(filename, settings)
        else:
            self.__load_config(filename)

    def __save_config(self, filename: str, settings: Settings):
        save_dict = {}

        if settings.is_server != DefaultValuesAndOptions.get_util_mods_options_data().default_value:
            save_dict[self.__IS_SERVER[0]] = settings.is_server

        if settings.socket_buffer_size != DefaultValuesAndOptions.get_socket_buffer_size_options_data().default_value:
            save_dict[self.__SOCKET_BUFFER_SIZE[0]] = settings.socket_buffer_size

        if (settings.audio_device.is_input_device != DefaultValuesAndOptions.get_audio_device_type_options_data()
                .default_value):
            save_dict[self.__IS_INPUT_DEVICE[0]] = settings.audio_device.is_input_device

        if settings.udp_port != DefaultValuesAndOptions.get_default_port():
            save_dict[self.__UDP_PORT[0]] = settings.udp_port

        if settings.password is not None:
            save_dict[self.__PASSWORD[0]] = settings.password

        if settings.reconnect_attempt != DefaultValuesAndOptions.get_default_reconnect_attempt():
            save_dict[self.__RECONNECT_ATTEMPT[0]] = settings.reconnect_attempt

        if settings.audio_device.device_id is not None:
            save_dict[self.__DEVICE_ID[0]] = settings.audio_device.device_id

        if (settings.audio_device.ignore_device_encoding_names != DefaultValuesAndOptions
                .get_ignore_encoding_options_data().default_value):
            save_dict[self.__IGNORE_DEVICE_ENCODING_NAMES[0]] = settings.audio_device.ignore_device_encoding_names

        if (settings.audio_device.device_encoding_names != DefaultValuesAndOptions.get_encoding_charset_options_data()
                .default_value):
            save_dict[self.__DEVICE_ENCODING_NAMES[0]] = settings.audio_device.device_encoding_names

        if settings.is_error_log != DefaultValuesAndOptions.get_error_log_options_data().default_value:
            save_dict[self.__IS_ERROR_LOG[0]] = settings.is_error_log

        if settings.is_server:
            if settings.crypto_manager.is_ssl != DefaultValuesAndOptions.get_ssl_options_data().default_value:
                save_dict[self.__IS_SSL[0]] = settings.crypto_manager.is_ssl

            if (settings.crypto_manager.is_integrity_control != DefaultValuesAndOptions.get_hash_control_options_data()
                    .default_value):
                save_dict[self.__IS_INTEGRITY_CONTROL[0]] = settings.crypto_manager.is_integrity_control
        else:
            save_dict[self.__SERVER_ADDRESS[0]] = settings.server_address

        if not filename.endswith('.conf'):
            filename += '.conf'

        with open(filename, 'w', encoding=locale.getpreferredencoding()) as file:
            file.write(f"[{DefaultValuesAndOptions.CONFIG_TITLE}]\n")
            for key, value in save_dict.items():
                file.write(f"{key}={value}\n")

        logging.info(f'Config file successfully saved in "{filename}"')

    def __load_config(self, filename: str):
        try:
            properties = self.__read_properties(filename)

            if self.__IS_ERROR_LOG[0] in properties:
                try:
                    bool_value = json.loads(properties[self.__IS_ERROR_LOG[0]].lower())
                    if bool_value:
                        Logger.init_warning_logger()
                except Exception:
                    raise ValueError(f"Invalid Boolean param in key {self.__IS_ERROR_LOG[0]}")

            self.__validate_keys(properties)
            properties = self.__validate_values(properties)
            self.__fill_class_field_with_default_values()

            for key, value in properties.items():
                setattr(self, key, value)
        except ValueError as e:
            logging.error(e)
            time.sleep(1)

            input("Press Enter to exit...")
            sys.exit(1)

    def __fill_class_field_with_default_values(self):
        class_fields = list(get_type_hints(self).keys())

        client_default_values = [config_key for config_key in ConfigParser.__CLIENT_PROPS]
        server_default_values = [config_key for config_key in ConfigParser.__SERVER_PROPS]

        all_default_values = [config_key for config_key in ConfigParser.__OTHER_PROPS]
        all_default_values.extend(client_default_values)
        all_default_values.extend(server_default_values)

        all_default_values = dict(all_default_values)

        for field in class_fields:
            setattr(self, field, all_default_values[field])

    def __validate_values(self, properties: Dict) -> Dict:
        class_fields = dict(get_type_hints(self).items())

        incorrect_values = []
        for key, value in properties.items():
            if class_fields[key] == bool:
                try:
                    bool_value = json.loads(value.lower())
                    if not isinstance(bool_value, bool):
                        incorrect_values.append(self.__format_invalid_value_type_error_str(key,
                                                                                           value,
                                                                                           "Boolean"))
                        continue
                except Exception:
                    incorrect_values.append(self.__format_invalid_value_type_error_str(key,
                                                                                       value,
                                                                                       "Boolean"))
                    continue

            if (class_fields[key] == int or class_fields[key] == Optional[int]) and (
                    value is not None and not value.isdigit()):
                incorrect_values.append(self.__format_invalid_value_type_error_str(key,
                                                                                   value,
                                                                                   "Integer"))

        if len(incorrect_values) > 0:
            raise ValueError(
                f"Next keys in config file has invalid type of value:{os.linesep}{os.linesep.join(incorrect_values)}"
            )
        else:
            for key, value in properties.items():
                if class_fields[key] == bool:
                    properties[key] = json.loads(value.lower())
                    continue

                if class_fields[key] == int or class_fields[key] == Optional[int]:
                    properties[key] = int(value)

        return properties

    @staticmethod
    def __format_invalid_value_type_error_str(key: str, value: str, need_type: str) -> str:
        return f'Value "{value}" of key "{key}" must be {need_type} type'

    @staticmethod
    def __validate_keys(properties: Dict):
        client_keys = [config_key[0] for config_key in ConfigParser.__CLIENT_PROPS]
        server_keys = [config_key[0] for config_key in ConfigParser.__SERVER_PROPS]

        valid_config_keys = [config_key[0] for config_key in ConfigParser.__OTHER_PROPS]
        valid_config_keys.extend(client_keys)
        valid_config_keys.extend(server_keys)

        properties_keys = list(properties.keys())
        bad_keys = []
        for key in properties_keys:
            if key not in valid_config_keys:
                bad_keys.append(key)

        if len(bad_keys) > 0:
            raise ValueError(f"Invalid keys in config file: {' ,'.join(bad_keys)}{os.linesep}"
                             f"List of valid keys: {os.linesep}{os.linesep.join(valid_config_keys)}")

        if ConfigParser.__IS_SERVER[0] in properties:
            is_server = json.loads(properties[ConfigParser.__IS_SERVER[0]].lower())

            if is_server:
                mode = 'Server'
                mode_keys = server_keys
                non_valid_mode_keys = ConfigParser.__validate_keys_in_cycle(properties_keys, client_keys)
            else:
                mode = 'Client'
                mode_keys = client_keys
                non_valid_mode_keys = ConfigParser.__validate_keys_in_cycle(properties_keys, server_keys)
        else:
            mode = 'Server'
            mode_keys = server_keys
            non_valid_mode_keys = ConfigParser.__validate_keys_in_cycle(properties_keys, client_keys)

        if len(non_valid_mode_keys) > 0:
            raise ValueError(
                f"Invalid keys in config file for {mode} mode: {' ,'.join(non_valid_mode_keys)}{os.linesep}"
                f"List of valid keys for {mode} mode: {os.linesep}{os.linesep.join(mode_keys)}")

    @staticmethod
    def __validate_keys_in_cycle(properties_keys: List, keys: List) -> List:
        non_valid_mode_keys = []
        for key in properties_keys:
            if key in keys:
                non_valid_mode_keys.append(key)

        return non_valid_mode_keys

    @staticmethod
    def __read_properties(file_path: str) -> Dict:
        if not os.path.isfile(file_path):
            raise ValueError(f'Config file "{file_path}" not found!')

        config = configparser.ConfigParser()
        file_str = ConfigParser.get_file_str(file_path)

        if file_str.split('\n')[0] != f'[{DefaultValuesAndOptions.CONFIG_TITLE}]':
            raise ValueError(f'Config file does not have title [{DefaultValuesAndOptions.CONFIG_TITLE}]')

        config.read_string(file_str)
        properties = dict(config[DefaultValuesAndOptions.CONFIG_TITLE])

        return properties

    @staticmethod
    def get_file_str(file_path: str) -> str:
        try:
            with open(file_path, 'r', encoding=locale.getpreferredencoding()) as file:
                file_str = file.read()
        except Exception as e:
            raise ValueError(f"Failed to decode config file: {e}")

        return file_str
