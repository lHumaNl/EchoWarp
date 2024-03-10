import argparse

from settings import Settings


def get_settings_by_args() -> Settings:
    parser = argparse.ArgumentParser(description="EchoWarp Audio Streamer")

    parser.add_argument("-s", "--server", action='store_true',
                        help="Start util in server mode. (default=Client mode")
    parser.add_argument("-o", "--output", action='store_false',
                        help="Use output audio device. (default=Use input audio device")
    parser.add_argument("-a", "--server_addr", type=str, help="Server address. Required for client mode.")
    parser.add_argument("-p", "--udp_port", type=int, default=Settings.get_default_port(),
                        help="UDP port for audio streaming.")
    parser.add_argument("-e", "--encoding", type=str, default=Settings.get_default_encoding(),
                        help=f"Device names encoding (default={Settings.get_default_encoding()}).")
    parser.add_argument("-b", "--heartbeat", type=int, default=Settings.get_default_heartbeat_attempt(),
                        help=f"Heartbeat attempt (default={Settings.get_default_heartbeat_attempt()}).")

    args = parser.parse_args()

    if args.server and args.server_addr:
        parser.error("-a/--server_addr is not allowed when -s/--server is specified.")

    return Settings(args.server, args.output, args.udp_port, args.server_addr, args.encoding, args.heartbeat)
