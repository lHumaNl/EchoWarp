import argparse
import os

from echowarp.logging_config import Logger
from echowarp.models.audio_device import AudioDevice
from echowarp.models.default_values_and_options import DefaultValuesAndOptions
from echowarp.settings import Settings
from echowarp.start_modes.config_parser import ConfigParser


class ArgsParser:
    @staticmethod
    def get_settings_from_cli_args() -> Settings:
        """
            Parses command line arguments to configure the EchoWarp utility settings. This function
            constructs the settings based on the provided command line arguments, handling different modes
            and configurations such as server/client mode, audio device selection, and network settings.

            Returns:
                Settings: An instance of the Settings class populated with values derived from parsed command line arguments.

            Raises:
                argparse.ArgumentError: If there are issues with the provided arguments, such as missing required arguments
                                        or invalid values.
        """
        parser = argparse.ArgumentParser(description="EchoWarp Audio Streamer")

        parser.add_argument("-c", "--client", action='store_false',
                            help=f"Start util in client mode. "
                                 f"(default={DefaultValuesAndOptions.get_util_mods_options_data().default_descr})")
        parser.add_argument(
            "-o", "--output", action='store_false',
            help=f"Use output audio device.{os.linesep}"
                 f"(default={DefaultValuesAndOptions.get_audio_device_type_options_data().default_descr})"
        )
        parser.add_argument("-u", "--udp_port", type=int, default=DefaultValuesAndOptions.get_default_port(),
                            help=f"UDP port for audio streaming. "
                                 f"(default={DefaultValuesAndOptions.get_default_port()})")
        parser.add_argument(
            "-b", "--socket_buffer_size", type=int,
            default=DefaultValuesAndOptions.get_socket_buffer_size_options_data().default_value,
            help=f"Size of socket buffer. "
                 f"(default={DefaultValuesAndOptions.get_socket_buffer_size_options_data().default_value})"
        )
        parser.add_argument("-d", "--device_id", type=int,
                            help="Specify the device ID to bypass interactive selection.")
        parser.add_argument("-p", "--password", type=str, help="Password, if needed.")
        parser.add_argument("-f", "--config_file", type=str, help="Path to config file "
                                                                  "(Ignoring other args, if they added).")
        parser.add_argument("-e", "--device_encoding_names", type=str,
                            help=f"Charset encoding for audio device. "
                                 f"(default=preferred system encoding - "
                                 f"{DefaultValuesAndOptions.get_encoding_charset_options_data().default_value})",
                            default=DefaultValuesAndOptions.get_encoding_charset_options_data().default_value)
        parser.add_argument("--ignore_device_encoding_names", action='store_true',
                            help="Ignoring device names encoding.")

        parser.add_argument("-l", "--is_error_log", action='store_true', help="Init error file logger.")
        parser.add_argument("-r", "--reconnect", type=int,
                            default=DefaultValuesAndOptions.get_default_reconnect_attempt(),
                            help=f"The number of failed connections in client mode before closing the application. "
                                 f"The number of failed client authorization attempts in server mode "
                                 f"before banning the client. (0 = infinite). "
                                 f"(default={DefaultValuesAndOptions.get_default_reconnect_attempt()})")

        parser.add_argument("-s", "--save_config", type=str, help="Save config file from selected args "
                                                                  "(Ignored default values)")

        client_args = parser.add_argument_group('Client Only Options')
        client_args.add_argument("-a", "--server_address", type=str, help="Server address.")

        server_args = parser.add_argument_group('Server Only Options')
        server_args.add_argument("--ssl", action='store_true',
                                 help=f"Init SSL (Encryption) mode (server mode only). "
                                      f"(default={DefaultValuesAndOptions.get_ssl_options_data().default_descr})")
        server_args.add_argument(
            "-i", "--integrity_control", action='store_true',
            help=f"Init integrity control of sending data (server mode only). "
                 f"(default={DefaultValuesAndOptions.get_hash_control_options_data().default_descr})"
        )
        server_args.add_argument("-w", "--workers", type=int,
                                 help="Max workers in multithreading (server mode only).",
                                 default=DefaultValuesAndOptions.get_default_workers())

        args = parser.parse_args()

        if args.is_error_log:
            Logger.init_warning_logger()

        if args.config_file is not None:
            return ArgsParser.__load_from_config(args.config_file)

        if not args.client:
            if any([args.ssl, args.integrity_control]) and args.workers != 1:
                parser.error("Options --ssl, --integrity_control and --workers are only available in server mode.")
            if not args.server_address:
                parser.error("The --server_address argument is required in client mode.")
        else:
            if args.server_address:
                parser.error("The --server_address argument is only valid in client mode.")

        if args.ignore_device_encoding_names:
            args.device_encoding_names = DefaultValuesAndOptions.get_encoding_charset_options_data().default_value

        audio_device = AudioDevice(args.output,
                                   args.device_id,
                                   args.ignore_device_encoding_names,
                                   args.device_encoding_names)

        settings = Settings(
            args.client,
            args.udp_port,
            args.server_address,
            args.reconnect,
            args.ssl,
            args.integrity_control,
            args.workers,
            audio_device,
            args.password,
            args.is_error_log,
            args.socket_buffer_size
        )

        if args.save_config is not None:
            ConfigParser(filename=args.save_config, settings=settings)

        return settings

    @staticmethod
    def __load_from_config(filepath: str) -> Settings:
        configs = ConfigParser(filename=filepath)
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
