import socket
import logging
import threading
import time

from logging_config import setup_logging

setup_logging()


class TCPClient:
    __client_socket: socket
    __server_address: str
    __udp_port: int
    __heartbeat_attempt: int
    __stop_event: threading.Event

    def __init__(self, server_address: str, udp_port: int, heartbeat_attempt: int, stop_event: threading.Event):
        self.__server_address = server_address
        self.__udp_port = udp_port
        self.__heartbeat_attempt = heartbeat_attempt
        self.__stop_event = stop_event


    def start_tcp_client(self):
        self.__client_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)

        try:
            self.__client_socket.connect((self.__server_address, self.__udp_port))
        except Exception as e:
            self.__client_socket.close()
            logging.error(f"Failed to established connect to {self.__server_address}:{self.__udp_port}: {e}")
            raise Exception

        try:
            self.__server_handler()
        except Exception as e:
            self.__client_socket.close()
            logging.error(f"Connection error: {e}")
            raise Exception

    def __server_handler(self):
        logging.info(f"TCP connection to {self.__server_address}:{self.__udp_port} established.")

        if self.__client_socket.recv(1024) == b"EchoWarpServer":
            self.__client_socket.sendall(b"EchoWarpClient")
            logging.info("Server authenticated")
        else:
            raise ValueError("Authentication error")

        threading.Thread(target=self.__heartbeat).start()

    def __heartbeat(self):
        heartbeat_fails = 0

        while not self.__stop_event.is_set():
            if heartbeat_fails < 0:
                heartbeat_fails = 0

            if heartbeat_fails > self.__heartbeat_attempt:
                raise ValueError(f"Heartbeat fails is too many: {heartbeat_fails}")

            self.__client_socket.sendall(b"heartbeat")
            if self.__client_socket.recv(1024) == b"heartbeat":
                if heartbeat_fails >= 1:
                    heartbeat_fails -= 1

                time.sleep(5)
            else:
                heartbeat_fails += 1
                logging.warning(
                    f"Missed 'heartbeat' from {self.__server_address}:{self.__udp_port}. "
                    f"Missed 'heartbeat': {heartbeat_fails}")
