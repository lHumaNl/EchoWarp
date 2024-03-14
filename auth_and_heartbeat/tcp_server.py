import logging
import socket
import threading
import time

from crypto_utils import CryptoManager
from logging_config import setup_logging

setup_logging()


class TCPServer:
    client_addr: str
    __client_connect: socket
    __udp_port: int
    __server_socket: socket
    __heartbeat_attempt: int
    __stop_event: threading.Event
    crypto_manager: CryptoManager

    def __init__(self, udp_port, heartbeat_attempt: int, stop_event: threading.Event, crypto_manager: CryptoManager):
        self.__udp_port = udp_port
        self.__heartbeat_attempt = heartbeat_attempt
        self.__stop_event = stop_event
        self.crypto_manager = crypto_manager

    def start_tcp_server(self):
        self.__server_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self.__server_socket.bind(('0.0.0.0', self.__udp_port))
        self.__server_socket.listen()
        logging.info(f'TCP server started at port "{self.__udp_port}" and await for client')

        try:
            self.__client_connect, client_addr = self.__server_socket.accept()
            self.client_addr = client_addr[0]
        except Exception as e:
            self.__server_socket.close()
            logging.error(f"Server error: {e}")
            raise Exception

        try:
            self.__client_handler()
        except Exception as e:
            self.__server_socket.close()
            logging.error(f"Server error: {e}")
            raise Exception

    def __heartbeat(self):
        heartbeat_fails = 0

        while not self.__stop_event.is_set():
            if heartbeat_fails < 0:
                heartbeat_fails = 0

            if heartbeat_fails > self.__heartbeat_attempt:
                raise ValueError(f"Heartbeat fails is too many: {heartbeat_fails}")

            if self.__client_connect.recv(1024) == b"heartbeat":
                self.__client_connect.sendall(b"heartbeat")
                if heartbeat_fails >= 1:
                    heartbeat_fails -= 1

                time.sleep(5)
            else:
                heartbeat_fails += 1
                logging.warning(f"Missed 'heartbeat' from {self.client_addr}. Missed 'heartbeat': {heartbeat_fails}")

    def __client_handler(self):
        logging.info(f"Success connect to {self.client_addr}")
        self.__client_connect.sendall(b"EchoWarpServer")

        if self.__client_connect.recv(1024) == b"EchoWarpClient":
            logging.info(f"Client {self.client_addr} authenticated")

            self.crypto_manager.generate_aes_key_and_iv()
            aes_key_iv_package = self.crypto_manager.get_aes_key_and_iv()
            self.__client_connect.sendall(aes_key_iv_package)

            threading.Thread(target=self.__heartbeat).start()
        else:
            raise ValueError("Authentication error")
