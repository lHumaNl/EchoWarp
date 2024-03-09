import argparse

from settings import Settings


def get_settings_by_args() -> Settings:
    parser = argparse.ArgumentParser(description="EchoWarp Audio Streamer")
    mode_group = parser.add_mutually_exclusive_group(required=True)
    mode_group.add_argument("-c", "--client", action='store_true', help="Start util in client mode.")
    mode_group.add_argument("-s", "--server", action='store_true', help="Start util in server mode.")
    parser.add_argument("-a", "--server_addr", type=str, help="Server address. Required for client mode.")
    parser.add_argument("-p", "--udp_port", type=int, default=Settings.get_default_port(),
                        help="UDP port for audio streaming.")

    args = parser.parse_args()

    if args.client and not args.server_addr:
        parser.error("-a/--server_addr is required when -c/--client is specified.")
    elif args.server and args.server_addr:
        parser.error("-a/--server_addr is not allowed when -s/--server is specified.")

    return Settings(args.server, args.udp_port, args.server_addr)
