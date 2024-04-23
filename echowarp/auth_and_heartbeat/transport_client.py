import socket
import logging
import threading
import time
from abc import ABC
from typing import Optional

from ..auth_and_heartbeat.transport_base import TransportBase
from ..services.crypto_manager import CryptoManager
from ..models.json_message import JSONMessage, JSONMessageServer
from ..models.default_values_and_options import DefaultValuesAndOptions


class TransportClient(TransportBase, ABC):
    """
    Manages TCP client operations for EchoWarp, including connection establishment,
    server authentication, and secure data exchange.

    Attributes:
        _server_address (str): The IP address or hostname of the server.
    """
    _client_udp_socket: Optional[socket.socket]
    _server_address: str

    def __init__(self, server_address: str, udp_port: int, stop_util_event: threading.Event,
                 stop_stream_event: threading.Event, crypto_manager: CryptoManager):
        """
        Initializes the TCP client with the necessary configuration to establish a connection.

        Args:
            server_address (str): The server's IP address or hostname.
            udp_port (int): The TCP port on the server to connect to.
            stop_util_event (threading.Event): An event to signal the thread to stop operations.
            stop_stream_event (threading.Event): An event to signal the thread to stop operations.
            crypto_manager (CryptoManager): An instance to manage cryptographic operations.
        """
        super().__init__(udp_port, stop_util_event, stop_stream_event, None, crypto_manager)
        self._server_address = server_address
        self._client_udp_socket = None

        self._start_tcp_client()

    def _start_tcp_client(self):
        """
        Establishes a TCP connection to the server, authenticates and sets up the secure communication channel.

        Raises:
            ConnectionError: If there is an issue with connecting to the server.
            ValueError: If the server's response during authentication is invalid.
        """
        self._initialize_socket()

        try:
            self._established_connection(False)
        except socket.error as e:
            logging.error(f"Failed to establish connection to {self._server_address}:{self._tcp_port}: {e}")
            self._cleanup_client_sockets()
            raise ConnectionError(f"Failed to establish connection to {self._server_address}:{self._tcp_port}")
        except Exception as e:
            logging.error(f"Authentication or encryption setup failed: {e}")
            self._cleanup_client_sockets()

    def __authenticate_with_server(self):
        """
        Performs authentication with the server using RSA encryption for the exchange of credentials.

        Raises:
            ValueError: If the authentication with the server fails or if versions mismatch.
        """
        try:
            self._client_tcp_socket.sendall(self._crypto_manager.get_serialized_public_key())
            server_public_key_pem = self._client_tcp_socket.recv(DefaultValuesAndOptions.SOCKET_BUFFER_SIZE)
            self._crypto_manager.load_peer_public_key(server_public_key_pem)

            client_auth_message_bytes = JSONMessage.encode_message_to_json_bytes(
                JSONMessage.AUTH_CLIENT_MESSAGE,
                200
            )

            self._client_tcp_socket.sendall(self._crypto_manager.encrypt_rsa_message(client_auth_message_bytes))

            encrypted_message_from_server = self._client_tcp_socket.recv(DefaultValuesAndOptions.SOCKET_BUFFER_SIZE)
            message_from_server = self._crypto_manager.decrypt_rsa_message(encrypted_message_from_server)
            config_server_message = JSONMessageServer(message_from_server)

            if (config_server_message.message != JSONMessageServer.AUTH_SERVER_MESSAGE
                    or config_server_message.response_code != 200):
                logging.error(
                    f"Failed to authenticate server. Message from server: "
                    f"{config_server_message.response_code} {config_server_message.message}")
                raise ValueError("Server authentication failed.")

            if config_server_message.version != DefaultValuesAndOptions.get_util_version():
                logging.error(f"Client version not equal server version: "
                              f"{DefaultValuesAndOptions.get_util_version()} - Client, "
                              f"{config_server_message.version} - Server")
                raise ValueError("Version mismatch between client and server.")

            self._crypto_manager.load_aes_key_and_iv(config_server_message.aes_key,
                                                     config_server_message.aes_iv)
            self._crypto_manager.load_encryption_config_for_client(config_server_message.is_encrypt,
                                                                   config_server_message.is_integrity_control)
            self._heartbeat_attempt = config_server_message.heartbeat_attempt

            logging.info("Authentication and load config from server completed successfully.")
        except socket.error as e:
            logging.error(f"Failed to send/receive data: {e}")
            self._stop_util_event.set()
        except RuntimeError as e:
            logging.error(f"Failed to maintain connection: {e}")
            self._stop_util_event.set()
        except Exception as e:
            logging.error(f"Error during authentication: {e}")
            self._stop_util_event.set()

    def _initialize_socket(self):
        self._client_tcp_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)

        self._client_udp_socket = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
        self._client_udp_socket.bind((self._server_address, self._udp_port))
        self._client_udp_socket.settimeout(5.0)

    def _established_connection(self, is_reconnect=True):
        self._stop_stream_event.clear()
        if is_reconnect:
            self._client_tcp_socket.connect((self._server_address, self._tcp_port))
        else:
            while not self._stop_util_event.is_set():
                try:
                    self._client_tcp_socket.connect((self._server_address, self._tcp_port))
                    break
                except socket.error as e:
                    logging.error(f"Failed to connect to {self._server_address}:{self._tcp_port}: {e}")
                    time.sleep(5)

        logging.info(f"TCP connection to {self._server_address}:{self._tcp_port} established.")
        self.__authenticate_with_server()
        self._stop_stream_event.set()

    def _cleanup_client_sockets(self):
        super()._cleanup_client_sockets()

        try:
            if self._client_udp_socket:
                self._client_udp_socket.close()
        except socket.error as e:
            logging.error(f"Error closing client UDP socket: {e}")
