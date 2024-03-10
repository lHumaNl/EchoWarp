import socket
import logging
import threading
import time

from logging_config import setup_logging

setup_logging()


class TCPClient:
    def __init__(self):
        self.__start_tcp_client()
    def __start_tcp_client(self, server_address: str, udp_port: int, heartbeat_attempt: int,
                           stop_event: threading.Event()):
        client_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)

        try:
            client_socket.connect((server_address, udp_port))
        except Exception as e:
            client_socket.close()
            logging.error(f"Failed to established connect to {server_address}:{udp_port}: {e}")

        try:
            self.__server_handler(client_socket, heartbeat_attempt, stop_event)
        except Exception as e:
            client_socket.close()
            logging.error(f"Connection error: {e}")
            raise Exception

    def __server_handler(self, server_address: str, udp_port: int, client_socket, heartbeat_attempt: int,
                         stop_event: threading.Event()):
        logging.info(f"TCP connection to {server_address}:{udp_port} established.")

        if client_socket.recv(1024) == b"EchoWarpServer":
            client_socket.sendall(b"EchoWarpClient")
            logging.info("Server authenticated")
        else:
            raise ValueError("Authentication error")

        threading.Thread(target=self.__heartbeat,
                         args=(server_address, udp_port, client_socket, heartbeat_attempt, stop_event)).start()

    def __heartbeat(self, server_address: str, udp_port: int, client_socket, heartbeat_attempt: int,
                    stop_event: threading.Event()):
        heartbeat_fails = 0

        while not stop_event.is_set():
            if heartbeat_fails < 0:
                heartbeat_fails = 0

            if heartbeat_fails > heartbeat_attempt:
                raise ValueError(f"Heartbeat fails is too many: {heartbeat_fails}")

            client_socket.sendall(b"heartbeat")
            if client_socket.recv(1024) == b"heartbeat":
                if heartbeat_fails >= 1:
                    heartbeat_fails -= 1

                time.sleep(5)
            else:
                heartbeat_fails += 1
                logging.warning(
                    f"Missed 'heartbeat' from {server_address}:{udp_port}. Missed 'heartbeat': {heartbeat_fails}")
