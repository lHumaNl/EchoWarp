import logging
import socket
import threading
from abc import ABC
from typing import Optional

from echowarp.auth_and_heartbeat.transport_base import TransportBase
from echowarp.services.crypto_manager import CryptoManager
from echowarp.models.json_message import JSONMessageServer, JSONMessage
from echowarp.models.default_values_and_options import DefaultValuesAndOptions


class TransportServer(TransportBase, ABC):
    """
    Manages TCP server operations for EchoWarp, including handling client connections,
    authentication, and setting up a secure communication channel.

    Attributes:
        _client_addr (Optional[str]): IP address of the connected client.
        _tcp_server_socket (Optional[socket.socket]): Server TCP socket for accepting client connections.
        _udp_server_socket (Optional[socket.socket]): Server UDP socket for accepting client connections.
        _stop_util_event (threading.Event): Event to signal util to stop.
        _stop_stream_event (threading.Event): Event to signal stream to stop.
    """
    _client_addr: Optional[str]
    _tcp_server_socket: Optional[socket.socket]
    _udp_server_socket: Optional[socket.socket]

    def __init__(self, udp_port, heartbeat_attempt: int, stop_util_event: threading.Event,
                 stop_stream_event: threading.Event, crypto_manager: CryptoManager):
        """
        Initializes the TCPServer with specified configuration parameters.

        Args:
            udp_port (int): The port number on which the server listens for incoming TCP connections.
            heartbeat_attempt (int): The number of allowed missed heartbeats before considering the connection lost.
            stop_util_event (threading.Event): An event to signal the server to stop operations.
            stop_stream_event (threading.Event): An event to signal the server to stop operations.
            crypto_manager (CryptoManager): An instance to manage cryptographic operations.
        """
        super().__init__(udp_port, stop_util_event, stop_stream_event, heartbeat_attempt, crypto_manager)

        self._client_addr = None
        self._tcp_server_socket = None
        self._udp_server_socket = None

        self._start_tcp_server()

    def _start_tcp_server(self):
        """
        Starts the TCP server, listens for incoming connections, and handles client authentication and setup.
        """
        self._tcp_server_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self._udp_server_socket = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)

        self._tcp_server_socket.bind(('0.0.0.0', self._tcp_port))
        self._tcp_server_socket.listen(1)

        logging.info(f'TCP server started on port "{self._tcp_port}" awaiting client connection')

        try:
            self._established_connection(False)
        except Exception as e:
            logging.error(f"Server encountered an error: {e}")
            self._cleanup_client_sockets()
            self._cleanup_server_socket()

    def __authenticate_client(self):
        """
        Handles the authentication sequence with the client by exchanging encrypted messages and
        establishing encryption settings.

        Raises:
            ValueError: If client authentication fails or there is a version mismatch.
        """
        try:
            self._client_tcp_socket.sendall(self._crypto_manager.get_serialized_public_key())

            client_public_key_pem = self._client_tcp_socket.recv(DefaultValuesAndOptions.SOCKET_BUFFER_SIZE)
            self._crypto_manager.load_peer_public_key(client_public_key_pem)

            encrypted_message_from_client = self._client_tcp_socket.recv(DefaultValuesAndOptions.SOCKET_BUFFER_SIZE)
            message_from_client = self._crypto_manager.decrypt_rsa_message(encrypted_message_from_client)

            client_auth_message = JSONMessage(message_from_client)

            if (client_auth_message.message != JSONMessage.AUTH_CLIENT_MESSAGE
                    or client_auth_message.response_code != 200):
                error_message = "Forbidden"
                error_message_bytes = JSONMessage.encode_message_to_json_bytes(error_message, 403)

                self._client_tcp_socket.sendall(self._crypto_manager.encrypt_rsa_message(error_message_bytes))

                logging.error(f"Failed to authenticate client. Client sent message: {client_auth_message.message}")
                raise ValueError("Client authentication failed.")

            logging.info("Client authenticated")

            if client_auth_message.version != DefaultValuesAndOptions.get_util_version():
                error_message = (f"Client version not equal server version: "
                                 f"{client_auth_message.version} - Client, "
                                 f"{DefaultValuesAndOptions.get_util_version()} - Server")
                error_message_bytes = JSONMessage.encode_message_to_json_bytes(error_message, 500)

                self._client_tcp_socket.sendall(self._crypto_manager.encrypt_rsa_message(error_message_bytes))

                logging.error(error_message)
                raise ValueError("Version mismatch between client and server.")

            self.__send_configuration()
        except socket.error as e:
            logging.error(f"Failed to send/receive data: {e}")
            self._stop_util_event.set()
        except RuntimeError as e:
            logging.error(f"Failed to maintain connection: {e}")
            self._stop_util_event.set()
        except Exception as e:
            logging.error(f"Error during authentication: {e}")
            self._stop_util_event.set()

    def __send_configuration(self):
        """
        Sends the configuration settings to the connected client, including security settings and AES keys.

        The configuration is sent as an encrypted JSON string.
        """
        config_json = JSONMessageServer.encode_server_config_to_json_bytes(
            self._heartbeat_attempt, self._crypto_manager.is_encrypt, self._crypto_manager.is_hash_control,
            self._crypto_manager.get_aes_key(), self._crypto_manager.get_aes_iv()
        )

        self._client_tcp_socket.sendall(self._crypto_manager.encrypt_rsa_message(config_json))
        logging.info("Configuration sent to client.")

    def _initialize_socket(self):
        pass

    def _established_connection(self, is_reconnect=True):
        self._tcp_server_socket.settimeout(5.0)
        is_connected = False
        while not self._stop_util_event.is_set():
            try:
                self._stop_stream_event.clear()
                self._client_tcp_socket, client_addr = self._tcp_server_socket.accept()
            except socket.timeout:
                continue

            self._client_addr = client_addr[0]

            logging.info(f"Client connected from {self._client_addr}")

            self.__authenticate_client()
            is_connected = True
            break

        self._stop_stream_event.set()

        if not is_connected:
            raise RuntimeError

    def _cleanup_server_socket(self):
        try:
            if self._tcp_server_socket:
                self._tcp_server_socket.shutdown(socket.SHUT_RDWR)
                self._tcp_server_socket.close()
                self._tcp_server_socket = None
        except socket.error as e:
            logging.error(f"Error closing server socket: {e}")
        except Exception as e:
            logging.error(f"Unhandled exception during server socket cleanup: {e}")
