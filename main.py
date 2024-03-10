import sys
import threading
import logging

import args_mode
import interactive_mode
from logging_config import setup_logging

from audio_server import start_server
from audio_client import start_client
from tcp_client import TCPClient
from tcp_server import TCPServer

setup_logging()


def main():
    if len(sys.argv) == 1:
        settings = interactive_mode.get_settings_interactive()
    else:
        settings = args_mode.get_settings_by_args()

    stop_event = threading.Event()

    if settings.is_server:
        logging.info("Start EchoWarp in server mode")
        tcp_server = TCPServer(settings.udp_port, settings.heartbeat_attempt, stop_event)

        start_server(settings.udp_port, stop_event)
    else:
        logging.info("Start EchoWarp in client mode")
        tcp_client = TCPClient(settings.server_addr, settings.udp_port, settings.heartbeat_attempt, stop_event)

        start_client(settings.server_addr, settings.udp_port, stop_event)

    try:
        input("Press Enter to exit...\n")
    finally:
        stop_event.set()


if __name__ == "__main__":
    main()
