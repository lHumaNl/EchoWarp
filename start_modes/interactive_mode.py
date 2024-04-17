import os

from prompt_toolkit import prompt
from prompt_toolkit.validation import Validator, ValidationError
import logging

from settings.audio_device import AudioDevice
from start_modes.default_values_and_options import DefaultValuesAndOptions
from settings.settings import Settings
from start_modes.options_data_creater import OptionsData


class NumberValidator(Validator):
    """
        A custom validator for prompt_toolkit that ensures user input is a valid number within a specified range.

        Attributes:
            valid_numbers (set): A set of numbers that are considered valid for input.
    """
    def __init__(self, valid_numbers):
        """
        Initializes the NumberValidator with the specified set of valid numbers.

        Args:
            valid_numbers (set): The set of valid numbers.
        """
        self.valid_numbers = valid_numbers

    def validate(self, document):
        """
        Validates the user input against the allowed numbers.

        Args:
            document (Document): The input document to validate.

        Raises:
            ValidationError: If the input is not a valid number or not in the allowed set.
        """
        text = document.text
        if not text:
            return None

        if not text.isdigit() or int(text) not in self.valid_numbers:
            raise ValidationError(
                message="Please enter a valid number",
                cursor_position=len(text))


class InteractiveSettings:
    """
    Handles the interactive configuration of EchoWarp settings through command line prompts.
    Allows users to configure settings such as server/client mode, audio device selection, and network options.
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
            server_address = prompt("Select server host: ")
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

        is_thread_mode = InteractiveSettings.__select_in_interactive_from_values(
            'workers mode',
            DefaultValuesAndOptions.get_thread_mode_options_data()
        )

        workers_count = InteractiveSettings.__input_in_interactive_int_value(
            DefaultValuesAndOptions.get_default_workers(), 'thread/process workers count')

        audio_device = AudioDevice(is_input_device, None)

        return Settings(is_server_mode, udp_port, server_address, heartbeat_attempt, is_ssl,
                        is_hash_control, is_thread_mode, workers_count, audio_device)

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
                       f"Your choice (default={options_data.default_descr}): ")
        while True:
            try:
                choice = prompt(prompt_text, validator=NumberValidator(options_dict.keys()))

                if not choice:
                    return options_data.default_value
                else:
                    return options_dict[int(choice)].option_value
            except ValidationError as ve:
                logging.error(f"Error: {ve}")
                print("Invalid input, please try again.")

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
                value = prompt(f"Select {descr} (default={default_value}): ")

                return int(value) if value else default_value
            except ValueError as ve:
                logging.error(f"Error: {ve}")
                print("Invalid input, please enter a valid integer.")
