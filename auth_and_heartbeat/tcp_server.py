import logging
import socket
import threading
from typing import Optional

from auth_and_heartbeat.tcp_base import TCPBase
from crypto_manager import CryptoManager
from logging_config import setup_logging
from settings.json_message import JSONMessageServer, JSONMessage
from start_modes.default_values_and_options import DefaultValuesAndOptions

setup_logging()


class TCPServer(TCPBase):
    """
    Manages TCP server operations for EchoWarp, including handling client connections,
    authentication, and setting up a secure communication channel.

    Attributes:
        client_addr (Optional[str]): IP address of the connected client.
        _server_socket (Optional[socket.socket]): Server socket for accepting client connections.
    """
    client_addr: Optional[str]
    _server_socket: Optional[socket.socket]

    def __init__(self, udp_port, heartbeat_attempt: int, stop_event: threading.Event, crypto_manager: CryptoManager):
        """
        Initializes the TCPServer with specified configuration parameters.

        Args:
            udp_port (int): The port number on which the server listens for incoming TCP connections.
            heartbeat_attempt (int): The number of allowed missed heartbeats before considering the connection lost.
            stop_event (threading.Event): An event to signal the server to stop operations.
            crypto_manager (CryptoManager): An instance to manage cryptographic operations.
        """
        super().__init__(udp_port, stop_event, heartbeat_attempt, crypto_manager)

        self.client_addr = None
        self._server_socket = None

    def start_tcp_server(self):
        """
        Starts the TCP server, listens for incoming connections, and handles client authentication and setup.
        """
        self._server_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)

        self._server_socket.bind(('0.0.0.0', self._udp_port))
        self._server_socket.listen(1)
        logging.info(f'TCP server started on port "{self._udp_port}" awaiting client connection')

        try:
            self._established_connection()
            logging.info(f"Client connected from {self.client_addr}")

            self.__authenticate_client()
        except Exception as e:
            logging.error(f"Server encountered an error: {e}")
        finally:
            if self._client_socket:
                self._client_socket.close()
            self._server_socket.close()

    def __authenticate_client(self):
        """
        Handles the authentication sequence with the client by exchanging encrypted messages and
        establishing encryption settings.

        Raises:
            ValueError: If client authentication fails or there is a version mismatch.
        """
        try:
            self._client_socket.sendall(self._crypto_manager.get_serialized_public_key())

            client_public_key_pem = self._client_socket.recv(DefaultValuesAndOptions.SOCKET_BUFFER_SIZE)
            self._crypto_manager.load_peer_public_key(client_public_key_pem)

            encrypted_message_from_client = self._client_socket.recv(DefaultValuesAndOptions.SOCKET_BUFFER_SIZE)
            message_from_client = self._crypto_manager.decrypt_rsa_message(encrypted_message_from_client)

            client_auth_message = JSONMessage(message_from_client)

            if (client_auth_message.message != JSONMessage.AUTH_CLIENT_MESSAGE
                    or client_auth_message.response_code != 200):
                error_message = "Forbidden"
                error_message_bytes = JSONMessage.encode_message_to_json_bytes(error_message, 403)

                self._client_socket.sendall(self._crypto_manager.encrypt_rsa_message(error_message_bytes))

                logging.error(f"Failed to authenticate client. Client sent message: {client_auth_message.message}")
                raise ValueError("Client authentication failed.")

            logging.info("Client authenticated")

            if client_auth_message.version != DefaultValuesAndOptions.get_util_version():
                error_message = (f"Client version not equal server version: "
                                 f"{client_auth_message.version} - Client, "
                                 f"{DefaultValuesAndOptions.get_util_version()} - Server")
                error_message_bytes = JSONMessage.encode_message_to_json_bytes(error_message, 500)

                self._client_socket.sendall(self._crypto_manager.encrypt_rsa_message(error_message_bytes))

                logging.error(error_message)
                raise ValueError("Version mismatch between client and server.")

            self.__send_configuration()
            # self._heartbeat()

            # threading.Thread(target=self._heartbeat, daemon=True).start()
        except Exception as e:
            logging.error(f"Authentication error: {e}")
            raise

    def __send_configuration(self):
        """
        Sends the configuration settings to the connected client, including security settings and AES keys.

        The configuration is sent as an encrypted JSON string.
        """
        config_json = JSONMessageServer.encode_server_config_to_json_bytes(
            self._heartbeat_attempt, self._crypto_manager.is_encrypt, self._crypto_manager.is_hash_control,
            self._crypto_manager.get_aes_key(), self._crypto_manager.get_aes_iv()
        )

        self._client_socket.sendall(self._crypto_manager.encrypt_rsa_message(config_json))
        logging.info("Configuration sent to client.")

    def _initialize_socket(self):
        return

    def _established_connection(self):
        self._client_socket, client_addr = self._server_socket.accept()
        self.client_addr = client_addr[0]
