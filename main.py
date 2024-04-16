import sys
import threading
import logging
import signal

from start_modes import args_mode
from logging_config import setup_logging
from start_modes.interactive_mode import InteractiveSettings

from streamer.audio_client import UDPClientStreamReceiver
from auth_and_heartbeat.tcp_client import TCPClient
from auth_and_heartbeat.tcp_server import TCPServer
from streamer.audio_server import UDPServerStreamer

setup_logging()
stop_event = threading.Event()


def graceful_shutdown(signum, frame):
    print("Gracefully shutting down...")
    stop_event.set()
    sys.exit(0)


def main():
    if len(sys.argv) == 1:
        settings = InteractiveSettings.get_settings_interactive()
    else:
        settings = args_mode.get_settings_by_args()

    if settings.is_server:
        logging.info("Start EchoWarp in server mode")
        tcp_server = TCPServer(settings.udp_port, settings.heartbeat_attempt, stop_event, settings.crypto_manager)
        tcp_server.start_tcp_server()

        streamer = UDPServerStreamer(tcp_server.client_addr, settings.udp_port, settings.audio_device, stop_event,
                                     settings.crypto_manager, settings.executor)
        streamer.encode_audio_and_send_to_client()
    else:
        logging.info("Start EchoWarp in client mode")
        tcp_client = TCPClient(settings.server_addr, settings.udp_port, settings.heartbeat_attempt, stop_event,
                               settings.crypto_manager)
        tcp_client.start_tcp_client()

        receiver = UDPClientStreamReceiver(settings.server_addr, settings.udp_port, settings.audio_device, stop_event,
                                           settings.crypto_manager, settings.executor)
        receiver.receive_audio_and_decode()

    try:
        input("Press Enter to exit...\n")
    finally:
        stop_event.set()


if __name__ == "__main__":
    signal.signal(signal.SIGINT, graceful_shutdown)
    main()
