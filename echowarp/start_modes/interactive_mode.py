import itertools
import os

import logging

from echowarp.logging_config import Logger
from echowarp.models.audio_device import AudioDevice
from echowarp.models.default_values_and_options import DefaultValuesAndOptions
from echowarp.settings import Settings
from echowarp.models.options_data_creater import OptionsData
from echowarp.start_modes.config_parser import ConfigParser


class NumberValidator:
    """
        A custom validator for prompt_toolkit that ensures user input is a valid number within a specified range.

        Attributes:
            valid_numbers (set): A set of numbers that are considered valid for input.
    """

    def __init__(self, valid_numbers: list):
        """
        Initializes the NumberValidator with the specified set of valid numbers.

        Args:
            valid_numbers (list): The set of valid numbers.
        """
        self.valid_numbers = valid_numbers

    def validate(self, input_str: str):
        """
        Validates the user input against the allowed numbers.

        Raises:
            ValidationError: If the input is not a valid number or not in the allowed set.
        """
        if input_str.strip() == '':
            return None

        if not input_str.isdigit() or int(input_str) not in self.valid_numbers:
            return False
        else:
            return True


class InteractiveSettings:
    """
    Handles the interactive configuration of EchoWarp settings through command line prompts.
    Allows users to configure settings such as server/client mode, audio device selection, and network options
    by answering interactive questions.
    """

    @staticmethod
    def get_settings_in_interactive_mode() -> Settings:
        """
        Prompts the user interactively to configure the EchoWarp settings and returns the configured settings object.

        Returns:
            Settings: A fully configured Settings object based on user input.
        """
        server_address = None
        is_ssl = None
        is_integrity_control = None
        workers = 1

        conf_files = []
        for filename in os.listdir():
            if os.path.isfile(os.path.join(filename)) and filename.endswith('.conf'):
                file_str = ConfigParser.get_file_str(filename)
                if file_str.split('\n')[0] == f'[{DefaultValuesAndOptions.CONFIG_TITLE}]':
                    conf_files.append(filename)

        if len(conf_files) == 1:
            is_load_config = InteractiveSettings.__select_in_interactive_from_values(
                f'load config from "{conf_files[0]}" file',
                DefaultValuesAndOptions.get_variants_config_load_options_data()
            )

            if is_load_config:
                return InteractiveSettings.__get_settings_from_config(conf_files[0])
        elif len(conf_files) > 1:
            config_files_num_list = [[value, i] for i, value in zip(itertools.count(), conf_files)]
            config_file_num = InteractiveSettings.__select_in_interactive_from_values(
                'founded config files',
                OptionsData(
                    ["Skip configs", False],
                    config_files_num_list
                )
            )

            if type(config_file_num) is int:
                selected_file_name = config_files_num_list[config_file_num][0]

                # noinspection PyTypeChecker
                return InteractiveSettings.__get_settings_from_config(selected_file_name)
        else:
            config_file_path = input("Input path to config file (empty field to skip): ")
            if config_file_path.strip() != '':
                return InteractiveSettings.__get_settings_from_config(config_file_path.strip())

        is_error_log = InteractiveSettings.__select_in_interactive_from_values(
            'error file logger switcher',
            DefaultValuesAndOptions.get_error_log_options_data()
        )

        if is_error_log:
            Logger.init_warning_logger()

        is_server_mode = InteractiveSettings.__select_in_interactive_from_values(
            'util mode',
            DefaultValuesAndOptions.get_util_mods_options_data()
        )

        socket_buffer_size = InteractiveSettings.__select_in_interactive_from_values(
            'size of socket buffer',
            DefaultValuesAndOptions.get_socket_buffer_size_options_data()
        )

        if type(socket_buffer_size) is bool:
            socket_buffer_size = InteractiveSettings.__get_not_null_int_input("custom buffer size in KB")

        is_input_device = InteractiveSettings.__select_in_interactive_from_values(
            'capture audio device type',
            DefaultValuesAndOptions.get_audio_device_type_options_data()
        )

        udp_port = InteractiveSettings.__input_in_interactive_int_value(
            DefaultValuesAndOptions.get_default_port(),
            'UDP port for audio streaming'
        )

        password = input("Input password, if needed: ")
        if password.strip() == '':
            password = None

        reconnect_attempt = InteractiveSettings.__input_in_interactive_int_value(
            DefaultValuesAndOptions.get_default_reconnect_attempt(),
            'count of reconnect attempt'
        )

        ignore_device_encoding_names = InteractiveSettings.__select_in_interactive_from_values(
            'ignore audio device encoding names',
            DefaultValuesAndOptions.get_ignore_encoding_options_data()
        )

        if ignore_device_encoding_names:
            device_encoding_names = InteractiveSettings.__select_in_interactive_from_values(
                'audio device charset encoding names',
                DefaultValuesAndOptions.get_encoding_charset_options_data()
            )

            if type(device_encoding_names) is bool:
                device_encoding_names = InteractiveSettings.__get_not_null_str_input('custom charset encoding')
        else:
            device_encoding_names = DefaultValuesAndOptions.get_encoding_charset_options_data().default_value

        if not is_server_mode:
            server_address = input("Select server host: ")
        else:
            is_ssl = InteractiveSettings.__select_in_interactive_from_values(
                'init ssl mode',
                DefaultValuesAndOptions.get_ssl_options_data()
            )

            is_integrity_control = InteractiveSettings.__select_in_interactive_from_values(
                'init integrity control',
                DefaultValuesAndOptions.get_hash_control_options_data()
            )

            workers = InteractiveSettings.__input_in_interactive_int_value(
                DefaultValuesAndOptions.get_default_workers(), 'thread workers count')

        audio_device = AudioDevice(is_input_device, None, ignore_device_encoding_names, device_encoding_names)
        settings = Settings(is_server_mode, udp_port, server_address, reconnect_attempt, is_ssl,
                            is_integrity_control, workers, audio_device, password, is_error_log, socket_buffer_size)

        save_to_config_file = InteractiveSettings.__select_in_interactive_from_values(
            'save selected values to config file',
            DefaultValuesAndOptions.get_save_profile_options_data()
        )

        if save_to_config_file:
            config_file_name = InteractiveSettings.__get_not_null_str_input('config file name')
            ConfigParser(filename=config_file_name, settings=settings)

        return settings

    @staticmethod
    def __get_not_null_str_input(descr: str) -> str:
        str_input = ''
        while str_input == '':
            str_input = input(f"Input {descr}: ").strip()
            if str_input == '':
                logging.error(f"Selected {descr} is NULL!")

        return str_input

    @staticmethod
    def __get_not_null_int_input(descr: str) -> str:
        int_input = ''
        while type(int_input) is not int:
            try:
                int_input = input(f'Input {descr}: ').strip()
                if int_input == '':
                    raise Exception
                int_input = int(int_input)
            except Exception:
                logging.error(f"Invalid {descr}: {int_input}")

        return int_input

    @staticmethod
    def __select_in_interactive_from_values(descr: str, options_data: OptionsData):
        """
        Displays a list of options to the user and allows them to select one interactively.

        Args:
            descr (str): Description of the setting being configured.
            options_data (OptionsData): Data containing the options and their descriptions.

        Returns:
            Any: The value of the selected option.
        """
        options_dict = {index: opt for index, opt in enumerate(options_data.options, start=1)}

        choices = os.linesep.join([f"{num}. {desc.option_descr}" for num, desc in options_dict.items()])
        prompt_text = (f"Select id of {descr}:"
                       f"{os.linesep + choices + os.linesep}"
                       f"Empty field for default value (default={options_data.default_descr}): {os.linesep}")

        number_validator = NumberValidator(list(options_dict.keys()))
        while True:
            try:
                choice = input(prompt_text)
                is_validating = number_validator.validate(choice)

                if is_validating is None:
                    return options_data.default_value
                elif is_validating:
                    return options_dict[int(choice)].option_value
                else:
                    raise ValueError
            except ValueError:
                logging.error(f"Invalid input, please try again.{os.linesep}"
                              f"Valid input id's: {number_validator.valid_numbers}")

    @staticmethod
    def __input_in_interactive_int_value(default_value: int, descr: str) -> int:
        """
        Prompts the user to input an integer value interactively, providing a default if no input is given.

        Args:
            default_value (int): The default value to use if no input is provided.
            descr (str): Description of the setting being configured.

        Returns:
            int: The user-input value or the default value.
        """
        while True:
            try:
                value = input(f"Select {descr} (default={default_value}): ")

                return int(value) if value else default_value
            except ValueError as ve:
                logging.error(f"Invalid input, please enter a valid integer: {ve}")

    @staticmethod
    def __get_settings_from_config(filepath: str) -> Settings:
        configs = ConfigParser(filepath)
        audio_device = AudioDevice(configs.is_input_audio_device,
                                   configs.device_id,
                                   configs.ignore_device_encoding_names,
                                   configs.device_encoding_names)

        return Settings(
            configs.is_server,
            configs.udp_port,
            configs.server_address,
            configs.reconnect_attempt,
            configs.is_ssl,
            configs.is_integrity_control,
            configs.workers,
            audio_device,
            configs.password,
            configs.is_error_log,
            configs.socket_buffer_size
        )
