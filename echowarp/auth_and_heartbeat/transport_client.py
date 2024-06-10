import os
import socket
import logging
import threading
from abc import ABC

from echowarp.auth_and_heartbeat.transport_base import TransportBase
from echowarp.models.json_message import JSONMessage, JSONMessageServer
from echowarp.models.default_values_and_options import DefaultValuesAndOptions
from echowarp.settings import Settings


class TransportClient(TransportBase, ABC):
    """
    Manages TCP client operations for EchoWarp, including connection establishment,
    server authentication, and secure data exchange.

    Attributes:
        _server_address (str): The IP address or hostname of the server.
    """
    _server_address: str

    def __init__(self, settings: Settings, stop_util_event: threading.Event(), stop_stream_event: threading.Event()):
        """
        Initializes the TCP client with the necessary configuration to establish a connection.

        Args:
            settings (Settings): Settings object.
        """
        super().__init__(settings, stop_util_event, stop_stream_event)
        self._server_address = settings.server_address

        self._init_tcp_connection()
        self._udp_socket.bind((self._server_address, self._udp_port))

    def __authenticate_on_server(self):
        """
        Performs authentication with the server using RSA encryption for the exchange of credentials.

        Raises:
            ValueError: If the authentication with the server fails or if versions mismatch.
        """

        self._client_tcp_socket.sendall(self._crypto_manager.get_serialized_public_key())
        server_public_key_pem = self._client_tcp_socket.recv(self._socket_buffer_size)
        self._crypto_manager.load_peer_public_key(server_public_key_pem)

        client_auth_message_bytes = JSONMessage.encode_message_to_json_bytes(
            self._password_base64,
            JSONMessage.OK_MESSAGE.response_code,
            None,
            None
        )

        self._client_tcp_socket.sendall(self._crypto_manager.encrypt_rsa_message(client_auth_message_bytes))

        encrypted_message_from_server = self._client_tcp_socket.recv(self._socket_buffer_size)
        message_from_server = self._crypto_manager.decrypt_rsa_message(encrypted_message_from_server)
        server_message = JSONMessage(message_from_server)
        config_server_message = None

        client_failed_connects = server_message.failed_connections
        reconnect_attempts = server_message.reconnect_attempts
        if reconnect_attempts is not None:
            client_failed_connects = f'{client_failed_connects}/{reconnect_attempts}'

        client_failed_connect_str = f'Client failed connect attempts on server: {client_failed_connects}'

        if (server_message.message == JSONMessage.OK_MESSAGE.response_message
                and server_message.response_code == JSONMessage.OK_MESSAGE.response_code):
            config_server_message = JSONMessageServer(message_from_server)

        elif (server_message.message == JSONMessage.FORBIDDEN_MESSAGE.response_message
              or server_message.response_code == JSONMessage.FORBIDDEN_MESSAGE.response_code):
            raise RuntimeError(f"Client is banned! Message from server: "
                               f"{server_message.response_code} {server_message.message}"
                               f"{os.linesep}{client_failed_connect_str}")

        elif (server_message.message == JSONMessage.UNAUTHORIZED_MESSAGE.response_message
              or server_message.response_code == JSONMessage.UNAUTHORIZED_MESSAGE.response_code):
            raise ValueError(f"Invalid password! Message from server: "
                             f"{server_message.response_code} {server_message.message}"
                             f"{os.linesep}{client_failed_connect_str}")

        if server_message.version != DefaultValuesAndOptions.get_util_comparability_version():
            raise ValueError(f"Client comparability version not equal server version: "
                             f"{DefaultValuesAndOptions.get_util_comparability_version()} - Client, "
                             f"{server_message.version} - Server"
                             f"{os.linesep}{client_failed_connect_str}")

        self._crypto_manager.load_aes_key_and_iv(config_server_message.aes_key_base64,
                                                 config_server_message.aes_iv_base64)
        self._crypto_manager.load_encryption_config_for_client(config_server_message.is_encrypt,
                                                               config_server_message.is_integrity_control)

        logging.info(f"Authentication on server completed.")

    def _initialize_socket(self):
        self._client_tcp_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self._client_tcp_socket.settimeout(DefaultValuesAndOptions.get_timeout())

    def _established_connection(self):
        self._client_tcp_socket.connect((self._server_address, self._udp_port))

        logging.info(f"TCP connection to {self._server_address} established.")

        try:
            self.__authenticate_on_server()
        except socket.timeout as e:
            raise socket.timeout(f"Timeout from server socket while authenticating: {e}")
        except socket.error as e:
            raise socket.error(f"Failed to send/receive data: {e}")
        except ValueError as e:
            raise ValueError(e)

    def _print_udp_listener_and_start_stream(self):
        logging.info(f'UDP listening started from {self._server_address}:{self._udp_port}')
        self._stop_stream_event.set()

    def _print_pause_udp_listener_and_pause_stream(self):
        logging.warning(f'Audio listening paused!')
        self._stop_stream_event.clear()
