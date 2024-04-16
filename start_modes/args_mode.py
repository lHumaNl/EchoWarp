import argparse

from settings.audio_device import AudioDevice
from start_modes.default_values_and_options import DefaultValuesAndOptions
from settings.settings import Settings


def get_settings_by_args() -> Settings:
    parser = argparse.ArgumentParser(description="EchoWarp Audio Streamer")

    parser.add_argument("-c", "--client", action='store_false',
                        help=f"Start util in client mode. "
                             f"(default={DefaultValuesAndOptions.get_util_mods_options_data().default_descr})")
    parser.add_argument("-o", "--output", action='store_false',
                        help=f"Use output audio device. "
                             f"(default={DefaultValuesAndOptions.get_audio_device_type_options_data().default_descr})")
    parser.add_argument("-p", "--udp_port", type=int, default=DefaultValuesAndOptions.get_default_port(),
                        help="UDP port for audio streaming.")
    parser.add_argument("-d", "--device_id", type=int,
                        help="Specify the device ID to bypass interactive selection.")
    parser.add_argument("-t", "--thread_mode", action='store_true',
                        help=f"Use {DefaultValuesAndOptions.get_thread_mode_options_data().options[0].option_descr}. "
                             f"(default={DefaultValuesAndOptions.get_thread_mode_options_data().default_descr})")
    parser.add_argument("-w", "--workers", type=int,
                        help="Max workers for multithreading/multiprocessing.",
                        default=DefaultValuesAndOptions.get_default_workers())

    client_args = parser.add_argument_group('Client Only Options')
    client_args.add_argument("-a", "--server_addr", type=str, help="Server address.")

    server_args = parser.add_argument_group('Server Only Options')
    server_args.add_argument("-b", "--heartbeat", type=int,
                             default=DefaultValuesAndOptions.get_default_heartbeat_attempt(),
                             help="Number of heartbeat attempts before disconnect (server mode only).")
    server_args.add_argument("--ssl", action='store_true',
                             help=f"Init SSL mode (server mode only). "
                                  f"(default={DefaultValuesAndOptions.get_ssl_options_data().default_descr})")
    server_args.add_argument("-i", "--integrity_control", action='store_true',
                             help=f"Init integrity control by hash (server mode only). "
                                  f"(default={DefaultValuesAndOptions.get_hash_control_options_data().default_descr})")

    args = parser.parse_args()

    if args.client:
        if args.server_addr:
            parser.error("The --server_addr argument is only valid in client mode.")
    else:
        if any([args.ssl, args.heartbeat, args.integrity_control]):
            parser.error("Options --ssl, --heartbeat, and --integrity_control are only available in server mode.")
        if not args.server_addr:
            parser.error("The --server_addr argument is required in client mode.")

    audio_device = AudioDevice(args.output, args.device_id)

    return Settings(
        args.client,
        args.udp_port,
        args.server_addr,
        args.heartbeat,
        args.ssl,
        args.integrity_control,
        args.thread_mode,
        args.workers,
        audio_device
    )
