import logging
import socket
import threading
import time
from abc import abstractmethod
from typing import Optional

from ..services.crypto_manager import CryptoManager
from ..models.json_message import JSONMessage
from ..models.default_values_and_options import DefaultValuesAndOptions


class TransportBase:
    """
        Provides base TCP functionality for both client and server sides of the EchoWarp application.
        Manages TCP connections, including sending heartbeats to maintain the connection and handling reconnections.

        Attributes:
            _client_tcp_socket (socket.socket): Socket for TCP communication.
            _tcp_port (int): TCP port used for communication, derived from UDP port.
            _udp_port (int): UDP port used for audio streaming.
            _stop_util_event (threading.Event): Event to signal when to terminate the application.
            _stop_stream_event (threading.Event): Event to signal when to stop streaming.
            _crypto_manager (CryptoManager): Manages cryptographic operations.
            _heartbeat_attempt (int, optional): Allowed missed heartbeats before connection is considered lost.
        """
    _client_tcp_socket: Optional[socket.socket]
    _tcp_port: int
    _udp_port: int
    _stop_util_event: threading.Event
    _stop_stream_event: threading.Event
    _crypto_manager: CryptoManager
    _heartbeat_attempt: Optional[int]

    def __init__(self, udp_port: int, stop_util_event: threading.Event, stop_stream_event: threading.Event,
                 heartbeat_attempt: Optional[int], crypto_manager: CryptoManager):
        """
        Initializes the TransportBase with provided configuration.

        Args:
            udp_port (int): UDP port for audio data streaming.
            stop_util_event (threading.Event): Event to manage application lifecycle.
            stop_stream_event (threading.Event): Event to manage streaming lifecycle.
            heartbeat_attempt (int, optional): Number of heartbeat attempts before termination.
            crypto_manager (CryptoManager): Manager for cryptographic operations.
        """
        self._tcp_port = udp_port - 1
        self._udp_port = udp_port
        self._stop_util_event = stop_util_event
        self._stop_stream_event = stop_stream_event
        self._heartbeat_attempt = heartbeat_attempt
        self._crypto_manager = crypto_manager
        self._client_tcp_socket = None

    def start_heartbeat(self, audio_thread: threading.Thread):
        """
        Continuously sends heartbeat messages to maintain the connection alive. If the heartbeat fails consecutively
        beyond a specified limit, the connection is considered lost and the thread signals the application to stop.

        Raises:
            RuntimeError: Raised if too many consecutive heartbeat messages are missed, indicating a connection issue.
        """
        audio_thread.start()
        heartbeat_interval = 5

        while not self._stop_util_event.is_set():
            try:
                heartbeat_message = JSONMessage.encode_message_to_json_bytes(JSONMessage.HEARTBEAT_MESSAGE, 200)
                self._client_tcp_socket.sendall(self._crypto_manager.encrypt_aes_and_sign_data(heartbeat_message))

                encrypted_response = self._client_tcp_socket.recv(DefaultValuesAndOptions.SOCKET_BUFFER_SIZE)
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
        try:
            logging.info("Attempting to stabilize the connection...")
            if self.__try_send_heartbeat():
                return

            self._cleanup_client_sockets()
            self._initialize_socket()

            self.__perform_reconnect_attempts()
        except Exception as e:
            logging.error(f"Critical error during heartbeat: {e}")
            self._stop_util_event.set()
            raise

    def __try_send_heartbeat(self):
        try:
            heartbeat_message = JSONMessage.encode_message_to_json_bytes(JSONMessage.HEARTBEAT_MESSAGE, 200)
            self._client_tcp_socket.sendall(self._crypto_manager.encrypt_aes_and_sign_data(heartbeat_message))
            logging.info("Connection stabilized with heartbeat.")
            return True
        except socket.error:
            logging.warning("Failed to send heartbeat.")
            return False

    def _cleanup_client_sockets(self):
        try:
            if self._client_tcp_socket:
                self._client_tcp_socket.shutdown(socket.SHUT_RDWR)
                self._client_tcp_socket.close()
                self._client_tcp_socket = None
        except socket.error as e:
            logging.error(f"Error closing client TCP socket: {e}")
        except Exception as e:
            logging.error(f"Unhandled exception during TCP socket cleanup: {e}")

    def __perform_reconnect_attempts(self):
        attempt = 0
        while not self._stop_util_event.is_set() and attempt < self._heartbeat_attempt:
            try:
                self._established_connection()
                logging.info(f"Reconnection successful on attempt {attempt + 1}")
                break
            except socket.error as e:
                attempt += 1
                logging.warning(
                    f"Reconnect failed due to socket error: {e}, retrying... "
                    f"(Attempt {attempt}/{self._heartbeat_attempt})")
                time.sleep(5)
            except Exception as e:
                attempt += 1
                logging.error(
                    f"Reconnect failed due to an unexpected error: {e}, retrying... "
                    f"(Attempt {attempt}/{self._heartbeat_attempt})")
                time.sleep(5)

        if attempt >= self._heartbeat_attempt:
            self._stop_util_event.set()
            logging.error("Maximum reconnect attempts reached, terminating connection.")
            raise RuntimeError("Connection lost due to missed reconnect attempts")

        self._print_listener()

    @abstractmethod
    def _print_listener(self):
        pass

    @abstractmethod
    def _initialize_socket(self):
        pass

    @abstractmethod
    def _established_connection(self, is_reconnect=True):
        pass
