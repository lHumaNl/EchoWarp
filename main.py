import sys
import threading
import logging

from start_modes import args_mode, interactive_mode
from logging_config import setup_logging

from streamer.audio_client import UDPClientStreamReceiver
from auth_and_heartbeat.tcp_client import TCPClient
from auth_and_heartbeat.tcp_server import TCPServer
from streamer.audio_server import UDPServerStreamer

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
        tcp_server.start_tcp_server()

        udp_server_streamer = UDPServerStreamer(tcp_server.client_addr, settings.udp_port, settings.audio_device,
                                                stop_event)
        udp_server_streamer.start_upd_server_streamer()
    else:
        logging.info("Start EchoWarp in client mode")
        tcp_client = TCPClient(settings.server_addr, settings.udp_port, settings.heartbeat_attempt, stop_event)
        tcp_client.start_tcp_client()

        start_udp_client_stream_receiver = UDPClientStreamReceiver(settings.server_addr, settings.udp_port,
                                                                   settings.audio_device, stop_event)
        start_udp_client_stream_receiver.start_udp_client_stream_receiver()

    try:
        input("Press Enter to exit...\n")
    finally:
        stop_event.set()


if __name__ == "__main__":
    main()
