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
        """
            Initiates a TCP client connection to the server and handles server authentication.

            Raises:
                ConnectionError: If the connection to the server cannot be established or lost.
                ValueError: If authentication with the server fails.
        """
        self.__client_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)

        try:
            self.__client_socket.connect((self.__server_address, self.__udp_port))
            self.__server_handler()
        except socket.error as e:
            logging.error(f"Failed to establish connection to {self.__server_address}:{self.__udp_port}: {e}")
            raise ConnectionError(f"Failed to establish connection to {self.__server_address}:{self.__udp_port}")
        finally:
            self.__client_socket.close()

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
        """
            Continuously sends heartbeat messages to the server to maintain the connection alive.

            Raises:
                RuntimeError: If too many heartbeat messages are missed, indicating a possible connection issue.
        """
        missed_heartbeats = 0

        while not self.__stop_event.is_set() and missed_heartbeats <= self.__heartbeat_attempt:
            try:
                self.__client_socket.sendall(b"heartbeat")
                if self.__client_socket.recv(1024) != b"heartbeat":
                    missed_heartbeats += 1
                    logging.warning(f"Heartbeat miss detected. Miss count: {missed_heartbeats}")
                else:
                    missed_heartbeats = max(0, missed_heartbeats - 1)
                time.sleep(5)
            except socket.error as e:
                logging.error(f"Heartbeat failed due to network error: {e}")
                break

        if missed_heartbeats > self.__heartbeat_attempt:
            logging.error("Too many heartbeat misses, terminating connection.")
            self.__stop_event.set()
