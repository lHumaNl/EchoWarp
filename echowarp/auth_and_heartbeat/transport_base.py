import base64
import logging
import os
import socket
import threading
import time
from abc import abstractmethod
from typing import Optional

from echowarp.models.default_values_and_options import DefaultValuesAndOptions
from echowarp.services.crypto_manager import CryptoManager
from echowarp.models.json_message import JSONMessage
from echowarp.settings import Settings


class TransportBase:
    """
    Provides base TCP functionality for both client and server sides of the EchoWarp application.
    Manages TCP connections, including sending heartbeats to maintain the connection and handling reconnections.

    Attributes:
        _is_server (bool): Indicates if the instance is running as a server.
        _client_tcp_socket (Optional[socket.socket]): Socket for TCP communication.
        _udp_socket (socket.socket): Socket for UDP communication.
        _tcp_port (int): TCP port used for communication, derived from UDP port.
        _udp_port (int): UDP port used for audio streaming.
        _stop_util_event (threading.Event): Event to signal when to terminate the application.
        _stop_stream_event (threading.Event): Event to signal when to stop streaming.
        _crypto_manager (CryptoManager): Manages cryptographic operations.
        _reconnect_attempt (int, optional): Allowed missed heartbeats before connection is considered lost.
        _password_base64 (Optional[str]): Base64 encoded password for authentication.
        _socket_buffer_size (int): Buffer size for the socket.
    """
    _is_server: bool
    _client_tcp_socket: Optional[socket.socket]
    _udp_socket: socket.socket
    _tcp_port: int
    _udp_port: int
    _stop_util_event: threading.Event
    _stop_stream_event: threading.Event
    _crypto_manager: CryptoManager
    _reconnect_attempt: int
    _password_base64: Optional[str]
    _socket_buffer_size: int

    def __init__(self, settings: Settings, stop_util_event: threading.Event(), stop_stream_event: threading.Event()):
        """
        Initializes the TransportBase with provided configuration.

        Args:
            settings (Settings): Settings object.
            stop_util_event (threading.Event): Event to signal when to terminate the application.
            stop_stream_event (threading.Event): Event to signal when to stop streaming.
        """
        self._is_server = settings.is_server
        self._tcp_port = settings.udp_port - 1
        self._udp_port = settings.udp_port
        self._stop_util_event = stop_util_event
        self._stop_stream_event = stop_stream_event
        self._reconnect_attempt = settings.reconnect_attempt
        self._crypto_manager = settings.crypto_manager

        self._client_tcp_socket = None
        self.__initialize_udp_socket()

        self._password_base64 = self.__get_base64_password(settings.password)

        self._socket_buffer_size = settings.socket_buffer_size

        self.__stop_message_send = False

    def start_streaming(self, audio_thread: threading.Thread):
        """
        Starts the streaming process by continuously sending heartbeat messages to maintain the connection alive.
        If the heartbeat fails consecutively beyond a specified limit, the connection is considered lost and the thread signals the application to stop.

        Args:
            audio_thread (threading.Thread): Thread for handling audio streaming.

        Raises:
            RuntimeError: Raised if too many consecutive heartbeat messages are missed, indicating a connection issue.
        """
        audio_thread.start()
        heartbeat_delay = DefaultValuesAndOptions.get_heartbeat_delay()

        while not self._stop_util_event.is_set():
            try:
                self.__send_and_get_heartbeat_message()

                time.sleep(heartbeat_delay)
            except ValueError as e:
                logging.error(e)
                self.__reconnect()
            except RuntimeError as e:
                logging.info(e)

                if self._is_server:
                    self.__reconnect(True)
                else:
                    break

            except (socket.error, socket.timeout) as e:
                logging.error(f"Heartbeat failed due to network error: {e}")
                self.__reconnect()
            except Exception as e:
                logging.error(f"Unexpected error during heartbeat: {e}")
                self.__reconnect()

        self._shutdown()

    def __send_and_get_heartbeat_message(self):
        """
        Sends and receives heartbeat messages for connection validation.
        """
        if self._is_server:
            self.__receive_heartbeat_and_validate()
            self.__format_and_send_heartbeat()
        else:
            self.__format_and_send_heartbeat()
            self.__receive_heartbeat_and_validate()

    @staticmethod
    def __get_base64_password(password: Optional[str]) -> Optional[str]:
        """
        Encodes the password to Base64 format.

        Args:
            password (Optional[str]): The password to encode.

        Returns:
            Optional[str]: The Base64 encoded password.
        """
        if password is None:
            return password
        else:
            return base64.b64encode(password.encode("utf-8")).decode("utf-8")

    def __receive_heartbeat_and_validate(self):
        """
        Receives and validates the heartbeat message.
        """
        encrypted_response = self._client_tcp_socket.recv(self._socket_buffer_size)
        if encrypted_response == b'':
            raise ValueError('Heartbeat message from client is NULL')

        decrypt_response = self._crypto_manager.decrypt_aes_and_verify_data(encrypted_response)
        response_message = JSONMessage(decrypt_response)

        if (response_message.message == JSONMessage.LOCKED_MESSAGE.response_message and
                response_message.response_code == JSONMessage.LOCKED_MESSAGE.response_code):
            raise RuntimeError("Received shutdown event. Close connection.")

        elif (response_message.message != JSONMessage.ACCEPTED_MESSAGE.response_message or
              response_message.response_code != JSONMessage.ACCEPTED_MESSAGE.response_code):
            raise ValueError(
                f"Response heartbeat message: {response_message.response_code} {response_message.message}")

    def __format_and_send_heartbeat(self):
        """
        Formats and sends a heartbeat message.
        """
        if self._stop_util_event.is_set():
            message = JSONMessage.encode_message_to_json_bytes(
                JSONMessage.LOCKED_MESSAGE.response_message,
                JSONMessage.LOCKED_MESSAGE.response_code,
                None,
                None
            )
            self.__stop_message_send = True
        else:
            message = JSONMessage.encode_message_to_json_bytes(
                JSONMessage.ACCEPTED_MESSAGE.response_message,
                JSONMessage.ACCEPTED_MESSAGE.response_code,
                None,
                None
            )

        self._client_tcp_socket.sendall(self._crypto_manager.encrypt_aes_and_sign_data(message))

    def __reconnect(self, is_client_exit=False):
        """
        Handles the reconnection logic for the transport layer.

        Args:
            is_client_exit (bool): Indicates if the client should exit.
        """
        self._print_pause_udp_listener_and_pause_stream()

        try:
            if not is_client_exit:
                logging.info("Attempting to stabilize the connection...")
                if self.__retry_send_heartbeat():
                    return

            self._cleanup_tcp_socket()

            if self._is_server:
                self._established_connection()
            else:
                self._initialize_socket()
                self.__perform_reconnect_attempts()

            if not self._stop_util_event.is_set():
                self._print_udp_listener_and_start_stream()
            else:
                self._shutdown()
        except RuntimeError as e:
            logging.error(e)
            self._shutdown()
        except Exception as e:
            raise Exception(f"Critical error during reconnect: {e}")

    def __retry_send_heartbeat(self) -> bool:
        """
        Retries sending the heartbeat message.

        Returns:
            bool: True if the heartbeat was successfully sent, False otherwise.
        """
        try:
            self.__send_and_get_heartbeat_message()
            logging.info("Connection stabilized with heartbeat.")

            return True
        except ValueError as e:
            logging.error(e)

            return False
        except (socket.error, socket.timeout) as e:
            logging.error(f"Failed to retry send heartbeat due socket exception: {e}")

            return False
        except Exception as e:
            logging.error(f"Unexpected exception due retry sending heartbeat: {e}")

            return False

    def __perform_reconnect_attempts(self):
        """
        Performs reconnection attempts until the maximum allowed attempts are reached.
        """
        attempt = 0
        reconnect_delay = DefaultValuesAndOptions.get_heartbeat_delay()

        while not self._stop_util_event.is_set() and attempt < self._reconnect_attempt:
            try:
                self._established_connection()
                logging.info(f"Reconnection successful on attempt {attempt + 1}")

                break
            except (socket.error, socket.timeout) as e:
                attempt += 1
                logging.error(f"Reconnect failed due to socket error:{os.linesep}{e}"
                              f"{self.__get_str_reconnect_attempt(attempt)}")
                time.sleep(reconnect_delay)
            except ValueError as e:
                attempt += 1
                logging.error(f"Reconnect failed due:{os.linesep}{e}"
                              f"{self.__get_str_reconnect_attempt(attempt)}")
                time.sleep(reconnect_delay)

        if attempt >= self._reconnect_attempt:
            self._shutdown()
            raise RuntimeError("Maximum reconnect attempts reached, terminating connection.")

    def __get_str_reconnect_attempt(self, attempt: int) -> str:
        """
        Returns a formatted string representing the reconnect attempt.

        Args:
            attempt (int): The current attempt number.

        Returns:
            str: Formatted string representing the reconnect attempt.
        """
        if self._reconnect_attempt == 0:
            return f"{os.linesep}Retrying connection... (Reconnect attempt: {attempt})"
        else:
            return f"{os.linesep}Retrying connection... (Reconnect attempt: {attempt}/{self._reconnect_attempt})"

    def _cleanup_tcp_socket(self):
        """
        Cleans up the TCP socket.
        """
        try:
            if self._client_tcp_socket:
                self._client_tcp_socket.shutdown(socket.SHUT_RDWR)
                self._client_tcp_socket.close()
        except socket.error as e:
            logging.error(f"Error closing client TCP socket: {e}")
        except Exception as e:
            logging.error(f"Unhandled exception during TCP socket cleanup: {e}")

    def _cleanup_udp_socket(self):
        """
        Cleans up the UDP socket.
        """
        try:
            if self._udp_socket:
                self._udp_socket.close()
        except socket.error as e:
            logging.error(f"Error closing client UDP socket: {e}")
        except Exception as e:
            logging.error(f"Unhandled exception during UDP socket cleanup: {e}")

    def _shutdown(self):
        """
        Shuts down the transport, signaling stop events and cleaning up sockets.
        """
        if not self.__stop_message_send:
            shutdown_message = JSONMessage.encode_message_to_json_bytes(
                JSONMessage.LOCKED_MESSAGE.response_message,
                JSONMessage.LOCKED_MESSAGE.response_code,
                None,
                None
            )
            self._client_tcp_socket.sendall(self._crypto_manager.encrypt_aes_and_sign_data(shutdown_message))

        self._stop_util_event.set()
        self._stop_stream_event.set()

        time.sleep(5)

        self._cleanup_tcp_socket()
        self._cleanup_udp_socket()

    def __initialize_udp_socket(self):
        """
        Initializes the UDP socket.
        """
        self._udp_socket = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
        self._udp_socket.settimeout(DefaultValuesAndOptions.get_timeout())

    def _init_tcp_connection(self):
        """
        Initializes the TCP connection.
        """
        self._initialize_socket()
        try:
            self._established_connection()
        except (socket.error, socket.timeout, RuntimeError, ValueError) as e:
            self._shutdown()
            raise RuntimeError(e)
        except Exception as e:
            self._shutdown()
            raise Exception(f"TCP connection failed: {e}")

    @abstractmethod
    def _print_udp_listener_and_start_stream(self):
        pass

    @abstractmethod
    def _print_pause_udp_listener_and_pause_stream(self):
        pass

    @abstractmethod
    def _initialize_socket(self):
        pass

    @abstractmethod
    def _established_connection(self):
        pass
