import os

import logging

from ..models.audio_device import AudioDevice
from ..models.default_values_and_options import DefaultValuesAndOptions
from ..settings import Settings
from ..models.options_data_creater import OptionsData


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
    def get_settings_interactive() -> Settings:
        """
        Prompts the user interactively to configure the EchoWarp settings and returns the configured settings object.

        Returns:
            Settings: A fully configured Settings object based on user input.
        """
        server_address = None
        heartbeat_attempt = None
        is_ssl = None
        is_hash_control = None
        workers_count = 1

        is_server_mode = InteractiveSettings.__select_in_interactive_from_values(
            'util mode',
            DefaultValuesAndOptions.get_util_mods_options_data()
        )

        is_input_device = InteractiveSettings.__select_in_interactive_from_values(
            'capture audio device type',
            DefaultValuesAndOptions.get_audio_device_type_options_data()
        )

        udp_port = InteractiveSettings.__input_in_interactive_int_value(
            DefaultValuesAndOptions.get_default_port(),
            'UDP port for audio streaming'
        )

        if not is_server_mode:
            server_address = input("Select server host: ")
        else:
            heartbeat_attempt = InteractiveSettings.__input_in_interactive_int_value(
                DefaultValuesAndOptions.get_default_heartbeat_attempt(),
                'count of heartbeat attempt'
            )

            is_ssl = InteractiveSettings.__select_in_interactive_from_values(
                'init ssl mode',
                DefaultValuesAndOptions.get_ssl_options_data()
            )

            is_hash_control = InteractiveSettings.__select_in_interactive_from_values(
                'init integrity control',
                DefaultValuesAndOptions.get_hash_control_options_data()
            )

            workers_count = InteractiveSettings.__input_in_interactive_int_value(
                DefaultValuesAndOptions.get_default_workers(), 'thread workers count')

        audio_device = AudioDevice(is_input_device, None)

        return Settings(is_server_mode, udp_port, server_address, heartbeat_attempt, is_ssl,
                        is_hash_control, workers_count, audio_device)

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
                       f"Your choice (default={options_data.default_descr}): {os.linesep}")

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
                logging.error(f"Error: {ve}")
                print("Invalid input, please enter a valid integer.")
