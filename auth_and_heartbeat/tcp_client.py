import socket
import logging
import threading

from auth_and_heartbeat.tcp_base import TCPBase
from crypto_manager import CryptoManager
from logging_config import setup_logging
from settings.json_message import JSONMessage, JSONMessageServer
from start_modes.default_values_and_options import DefaultValuesAndOptions

setup_logging()


class TCPClient(TCPBase):
    """
    Manages TCP client operations for EchoWarp, including connection establishment,
    server authentication, and secure data exchange.

    Attributes:
        _server_address (str): The IP address or hostname of the server.
    """
    _server_address: str

    def __init__(self, server_address: str, udp_port: int, stop_event: threading.Event, crypto_manager: CryptoManager):
        """
        Initializes the TCP client with the necessary configuration to establish a connection.

        Args:
            server_address (str): The server's IP address or hostname.
            udp_port (int): The TCP port on the server to connect to.
            stop_event (threading.Event): An event to signal the thread to stop operations.
            crypto_manager (CryptoManager): An instance to manage cryptographic operations.
        """
        super().__init__(udp_port, stop_event, None, crypto_manager)
        self._server_address = server_address

    def start_tcp_client(self):
        """
        Establishes a TCP connection to the server, authenticates and sets up the secure communication channel.

        Raises:
            ConnectionError: If there is an issue with connecting to the server.
            ValueError: If the server's response during authentication is invalid.
        """
        self._initialize_socket()

        try:
            self._established_connection()
            logging.info(f"TCP connection to {self._server_address}:{self._udp_port} established.")

            self.__authenticate_with_server()
        except socket.error as e:
            logging.error(f"Failed to establish connection to {self._server_address}:{self._udp_port}: {e}")
            self._cleanup_client_sockets()
            raise ConnectionError(f"Failed to establish connection to {self._server_address}:{self._udp_port}")
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
            self._client_socket.sendall(self._crypto_manager.get_serialized_public_key())
            server_public_key_pem = self._client_socket.recv(DefaultValuesAndOptions.SOCKET_BUFFER_SIZE)
            self._crypto_manager.load_peer_public_key(server_public_key_pem)

            client_auth_message_bytes = JSONMessage.encode_message_to_json_bytes(
                JSONMessage.AUTH_CLIENT_MESSAGE,
                200
            )

            self._client_socket.sendall(self._crypto_manager.encrypt_rsa_message(client_auth_message_bytes))

            encrypted_message_from_server = self._client_socket.recv(DefaultValuesAndOptions.SOCKET_BUFFER_SIZE)
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
            threading.Thread(target=self._heartbeat, daemon=True).start()
        except Exception as e:
            logging.error(f"Error during authentication: {e}")
            raise

    def _initialize_socket(self):
        self._client_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)

    def _established_connection(self):
        self._client_socket.connect((self._server_address, self._udp_port))
