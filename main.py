import sys
import threading
import logging

import args_mode
import interactive_mode
from logging_config import setup_logging

from tcp_server import start_tcp_server
from tcp_client import start_tcp_client
from audio_server import start_server
from audio_client import start_client

setup_logging()


def main():
    if len(sys.argv) == 1:
        settings = interactive_mode.get_settings_interactive()
    else:
        settings = args_mode.get_settings_by_args()

    stop_event = threading.Event()
    tcp_server_thread = None
    tcp_client_thread = None

    if settings.is_server:
        logging.info("Start EchoWarp in server mode")
        tcp_server_thread = threading.Thread(target=start_tcp_server, args=(settings.udp_port, stop_event))
        tcp_server_thread.start()

        start_server(settings.udp_port, stop_event)
    else:
        logging.info("Start EchoWarp in client mode")
        tcp_client_thread = threading.Thread(target=start_tcp_client,
                                             args=(settings.server_addr, settings.udp_port, stop_event))
        tcp_client_thread.start()

        start_client(settings.server_addr, settings.udp_port, stop_event)

    try:
        input("Press Enter to exit...\n")
    finally:
        stop_event.set()
        if settings.is_server:
            tcp_server_thread.join()
        else:
            tcp_client_thread.join()


if __name__ == "__main__":
    main()
