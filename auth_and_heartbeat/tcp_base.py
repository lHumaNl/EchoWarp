import logging
import socket
import threading
import time
from abc import abstractmethod
from typing import Optional

from logging_config import setup_logging

from crypto_manager import CryptoManager
from settings.json_message import JSONMessage
from start_modes.default_values_and_options import DefaultValuesAndOptions

setup_logging()


class TCPBase:
    """
        Provides base TCP functionality for both client and server sides of the EchoWarp application.
        This class manages the TCP connection, including sending heartbeats to keep the connection alive and
        handling potential connection losses.

        Attributes:
            _client_socket (socket.socket): Socket used for TCP communication.
            _udp_port (int): UDP port used for associated audio streaming.
            _stop_event (threading.Event): Event used to signal the termination of the server/client.
            _crypto_manager (CryptoManager): Instance for managing cryptographic operations.
            _heartbeat_attempt (int): Number of allowed missed heartbeats before considering the connection as lost.
    """
    _client_socket: Optional[socket.socket]
    _udp_port: int
    _stop_event: threading.Event
    _crypto_manager: CryptoManager
    _heartbeat_attempt: Optional[int]

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
        self._udp_port = udp_port - 1
        self._stop_event = stop_event
        self._heartbeat_attempt = heartbeat_attempt
        self._crypto_manager = crypto_manager
        self._client_socket = None

    def _heartbeat(self):
        """
        Continuously sends heartbeat messages to maintain the connection alive. If the heartbeat fails consecutively
        beyond a specified limit, the connection is considered lost and the thread signals the application to stop.

        Raises:
            RuntimeError: Raised if too many consecutive heartbeat messages are missed, indicating a connection issue.
        """
        heartbeat_interval = 5

        while not self._stop_event.is_set():
            try:
                heartbeat_message = JSONMessage.encode_message_to_json_bytes(JSONMessage.HEARTBEAT_MESSAGE, 200)
                self._client_socket.sendall(self._crypto_manager.encrypt_aes_and_sign_data(heartbeat_message))

                encrypted_response = self._client_socket.recv(DefaultValuesAndOptions.SOCKET_BUFFER_SIZE)
                decrypt_response = self._crypto_manager.decrypt_aes_and_verify_data(encrypted_response)
                response_message = JSONMessage(decrypt_response)

                if response_message.message != JSONMessage.HEARTBEAT_MESSAGE or response_message.response_code != 200:
                    raise ValueError(
                        f"Response heartbeat message: {response_message.response_code} {response_message.message}")

                time.sleep(heartbeat_interval)
            except ValueError as e:
                logging.error(e)
                self.__reconnect()
            except socket.error as e:
                logging.error(f"Heartbeat failed due to network error: {e}")
                self.__reconnect()
            except Exception as e:
                logging.error(f"Unexpected error during heartbeat: {e}")
                self.__reconnect()

    def __reconnect(self):
        logging.info("Attempting to stabilize the connection...")
        if self.__try_send_heartbeat():
            return

        self._cleanup_client_sockets()
        self._initialize_socket()

        self.__perform_reconnect_attempts()

    def __try_send_heartbeat(self):
        try:
            heartbeat_message = JSONMessage.encode_message_to_json_bytes(JSONMessage.HEARTBEAT_MESSAGE, 200)
            self._client_socket.sendall(self._crypto_manager.encrypt_aes_and_sign_data(heartbeat_message))
            logging.info("Connection stabilized with heartbeat.")
            return True
        except socket.error:
            logging.info("Failed to send heartbeat.")
            return False

    def _cleanup_client_sockets(self):
        try:
            if self._client_socket:
                self._client_socket.shutdown(socket.SHUT_RDWR)
                self._client_socket.close()
        except socket.error as e:
            logging.error(f"Error closing client socket: {e}")

    def __perform_reconnect_attempts(self):
        attempts = 0
        while not self._stop_event.is_set() and attempts < self._heartbeat_attempt:
            try:
                self._established_connection()
                logging.info("Reconnect successful")
                break
            except socket.error as e:
                attempts += 1
                logging.error(f"Reconnect failed: {e}, retrying... (Attempt {attempts}/{self._heartbeat_attempt})")
                time.sleep(5)

        if attempts >= self._heartbeat_attempt:
            logging.error("Maximum reconnect attempts reached, terminating connection.")
            self._stop_event.set()
            raise RuntimeError("Connection lost due to missed reconnect attempts")

    @abstractmethod
    def _initialize_socket(self):
        pass

    @abstractmethod
    def _established_connection(self):
        pass
