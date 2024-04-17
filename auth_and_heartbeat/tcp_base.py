import logging
import socket
import threading
import time
from typing import Optional

from logging_config import setup_logging

from crypto_manager import CryptoManager
from settings.json_message import JSONMessage

setup_logging()


class TCPBase:
    """
        Provides base TCP functionality for both client and server sides of the EchoWarp application.
        This class manages the TCP connection, including sending heartbeats to keep the connection alive and handling potential connection losses.

        Attributes:
            __client_socket (socket.socket): Socket used for TCP communication.
            __udp_port (int): UDP port used for associated audio streaming.
            __stop_event (threading.Event): Event used to signal the termination of the server/client.
            __crypto_manager (CryptoManager): Instance for managing cryptographic operations.
            __heartbeat_attempt (int): Number of allowed missed heartbeats before considering the connection as lost.
    """
    __client_socket: Optional[socket.socket]
    __udp_port: int
    __stop_event: threading.Event
    __crypto_manager: CryptoManager
    __heartbeat_attempt: Optional[int]

    def __init__(self, udp_port: int, stop_event: threading.Event, heartbeat_attempt: Optional[int],
                 crypto_manager: CryptoManager):
        """
                Initializes a TCPBase instance with the provided settings.

                Args:
                    udp_port (int): The UDP port for audio data streaming.
                    stop_event (threading.Event): Event to control the server/client lifecycle.
                    heartbeat_attempt (int, optional): Number of heartbeat attempts before termination.
                    crypto_manager (CryptoManager): Cryptographic manager for secure data handling.
        """
        self.__udp_port = udp_port
        self.__stop_event = stop_event
        self.__heartbeat_attempt = heartbeat_attempt
        self.__crypto_manager = crypto_manager
        self.__client_socket = None

    def __heartbeat(self):
        """
        Continuously sends heartbeat messages to maintain the connection alive. If the heartbeat fails consecutively
        beyond a specified limit, the connection is considered lost and the thread signals the application to stop.

        Raises:
            RuntimeError: Raised if too many consecutive heartbeat messages are missed, indicating a connection issue.
        """
        missed_heartbeats = 0
        heartbeat_interval = 5

        while not self.__stop_event.is_set() and missed_heartbeats <= self.__heartbeat_attempt:
            try:
                heartbeat_message = JSONMessage.encode_message_to_json_bytes(JSONMessage.HEARTBEAT_MESSAGE, 200)
                self.__client_socket.sendall(self.__crypto_manager.encrypt_aes_and_sign_data(heartbeat_message))

                encrypted_response = self.__client_socket.recv(1024)
                response_message = JSONMessage(self.__crypto_manager.decrypt_aes_and_verify_data(encrypted_response))

                if response_message.message != JSONMessage.HEARTBEAT_MESSAGE or response_message.response_code != 200:
                    missed_heartbeats += 1
                    logging.warning(f"Heartbeat miss detected. Miss count: {missed_heartbeats}")
                else:
                    missed_heartbeats = max(0, missed_heartbeats - 1)

                time.sleep(heartbeat_interval)
            except socket.error as e:
                logging.error(f"Heartbeat failed due to network error: {e}")
                missed_heartbeats += 1
            except Exception as e:
                logging.error(f"Unexpected error during heartbeat: {e}")
                missed_heartbeats += 1

            if missed_heartbeats > self.__heartbeat_attempt:
                logging.error("Too many heartbeat misses, terminating connection.")
                self.__stop_event.set()
                raise RuntimeError("Connection lost due to missed heartbeats")
