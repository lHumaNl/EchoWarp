import logging
import os
import socket
import threading
from abc import ABC
from typing import Optional

from echowarp.auth_and_heartbeat.transport_base import TransportBase
from echowarp.models.net_interfaces_info import NetInterfacesInfo
from echowarp.models.ban_list import BanList
from echowarp.models.json_message import JSONMessageServer, JSONMessage
from echowarp.models.default_values_and_options import DefaultValuesAndOptions
from echowarp.settings import Settings


class TransportServer(TransportBase, ABC):
    """
    Manages TCP server operations for EchoWarp, including handling client connections,
    authentication, and setting up a secure communication channel.

    Attributes:
        _client_address (Optional[str]): IP address of the connected client.
        _stop_util_event (threading.Event): Event to signal util to stop.
        _stop_stream_event (threading.Event): Event to signal stream to stop.
    """
    _server_tcp_socket: socket.socket
    _client_address: Optional[str]
    __ban_list: BanList
    __net_interfaces_info: NetInterfacesInfo

    def __init__(self, settings: Settings, stop_util_event: threading.Event(), stop_stream_event: threading.Event()):
        """
        Initializes the TCPServer with specified configuration parameters.

        Args:
            settings (Settings): Settings object.
        """
        super().__init__(settings, stop_util_event, stop_stream_event)

        self._client_address = None
        self.__ban_list = BanList(settings.reconnect_attempt)
        self.__net_interfaces_info = NetInterfacesInfo(self._udp_port)

        self._init_tcp_connection()

    def __authenticate_client(self):
        """
        Handles the authentication sequence with the client by exchanging encrypted messages and
        establishing encryption settings.

        Raises:
            ValueError: If client authentication fails or there is a version mismatch.
        """

        self._client_tcp_socket.sendall(self._crypto_manager.get_serialized_public_key())

        client_public_key_pem = self._client_tcp_socket.recv(self._socket_buffer_size)
        self._crypto_manager.load_peer_public_key(client_public_key_pem)

        encrypted_message_from_client = self._client_tcp_socket.recv(self._socket_buffer_size)
        message_from_client = self._crypto_manager.decrypt_rsa_message(encrypted_message_from_client)

        client_auth_message = JSONMessage(message_from_client)

        if self.__ban_list.is_banned(self._client_address):
            self.__form_error_message(
                JSONMessage.FORBIDDEN_MESSAGE.response_message,
                JSONMessage.FORBIDDEN_MESSAGE.response_code,
                f"Failed to authenticate client: {self._client_address}. "
                f"Client is {JSONMessage.FORBIDDEN_MESSAGE.response_message}"
            )

        if client_auth_message.message != self._password_base64:
            self.__form_error_message(
                JSONMessage.UNAUTHORIZED_MESSAGE.response_message,
                JSONMessage.UNAUTHORIZED_MESSAGE.response_code,
                f"Failed to authenticate client: {self._client_address}. "
                f"Client is {JSONMessage.UNAUTHORIZED_MESSAGE.response_message}"
            )

        if client_auth_message.version != DefaultValuesAndOptions.get_util_comparability_version():
            self.__form_error_message(
                JSONMessage.CONFLICT_MESSAGE.response_message,
                JSONMessage.CONFLICT_MESSAGE.response_code,
                f"Client version not equal server version: "
                f"{client_auth_message.version} - Client, "
                f"{DefaultValuesAndOptions.get_util_comparability_version()} - Server"
            )

        logging.info(f"Client {self._client_address} authenticated.")
        self.__ban_list.success_connect_attempt(self._client_address)
        self.__print_client_info()

        self.__send_configuration()

    def __form_error_message(self, response_message: str, response_code: int, exception_message: str):
        self.__ban_list.fail_connect_attempt(self._client_address)

        error_message_bytes = JSONMessage.encode_message_to_json_bytes(
            response_message,
            response_code,
            self.__ban_list.get_failed_connect_attempts(self._client_address),
            self._reconnect_attempt
        )

        self._client_tcp_socket.sendall(self._crypto_manager.encrypt_rsa_message(error_message_bytes))
        raise ValueError(exception_message)

    def __send_configuration(self):
        """
        Sends the configuration settings to the connected client, including security settings and AES keys.

        The configuration is sent as an encrypted JSON string.
        """
        config_json = JSONMessageServer.encode_server_config_to_json_bytes(
            self._crypto_manager.is_ssl, self._crypto_manager.is_integrity_control,
            self._crypto_manager.get_aes_key_base64(), self._crypto_manager.get_aes_iv_base64(),
            self.__ban_list.get_failed_connect_attempts(self._client_address), self._reconnect_attempt
        )

        self._client_tcp_socket.sendall(self._crypto_manager.encrypt_rsa_message(config_json))
        logging.info("Configuration sent to client.")

    def _initialize_socket(self):
        self._server_tcp_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self._server_tcp_socket.bind(('', self._udp_port))
        self._server_tcp_socket.settimeout(DefaultValuesAndOptions.get_timeout())
        self._server_tcp_socket.listen()

    def _established_connection(self):
        is_log = True
        while not self._stop_util_event.is_set():
            if is_log:
                logging.info(f'Authenticate server started and awaiting client connection on next INET interfaces:'
                             f'{os.linesep}{self.__net_interfaces_info.get_formatted_info_str()}')

            try:
                self._client_tcp_socket, client_address = self._server_tcp_socket.accept()
            except (socket.error, socket.timeout):
                is_log = False
                continue

            is_log = True
            self._client_address = client_address[0]
            self.__ban_list.add_client_to_ban_list(self._client_address)

            if self.__ban_list.is_banned(self._client_address) and not self.__ban_list.is_first_time_message(
                    self._client_address):
                logging.error(f'Client {self._client_address} banned!')
                self.__ban_list.fail_connect_attempt(self._client_address)
                continue

            logging.info(f"Client connected from {self._client_address}")

            try:
                self.__authenticate_client()
            except ValueError as e:
                logging.error(e)
                self.__print_client_info()
                continue
            except (socket.error, socket.timeout) as e:
                logging.error(f"Failed to send/receive data: {e}")
                self.__print_client_info()
                continue
            except Exception as e:
                logging.error(f"Error during authentication: {e}")
                self.__print_client_info()
                continue
            finally:
                self.__ban_list.update_ban_list_file()

            break

    def __print_client_info(self):
        success_connections = self.__ban_list.get_success_connect_attempts(self._client_address)
        failed_connections = self.__ban_list.get_failed_connect_attempts(self._client_address)
        all_failed_connections = self.__ban_list.get_all_failed_connect_attempts(self._client_address)

        failed_connections_attempts = f'{failed_connections}/{self._reconnect_attempt}' if self._reconnect_attempt > 0 \
            else failed_connections

        logging.info(
            f"Client {self._client_address} info:{os.linesep}"
            f"Success client connections: {success_connections}{os.linesep}"
            f"Failed client connection attempts after last success: {failed_connections_attempts}{os.linesep}"
            f"Count of failed client connections: {all_failed_connections}"
        )

    def _shutdown(self):
        super()._shutdown()
        try:
            if self._server_tcp_socket:
                self._server_tcp_socket.close()
        except socket.error as e:
            logging.error(f"Error closing server TCP socket: {e}")
        except Exception as e:
            logging.error(f"Unhandled exception during TCP socket cleanup: {e}")

    def _print_udp_listener_and_start_stream(self):
        logging.info(f"Start UDP audio streaming to client {self._client_address}:{self._udp_port}")
        self._stop_stream_event.set()

    def _print_pause_udp_listener_and_pause_stream(self):
        logging.warning(f'Audio streaming paused!')
        self._stop_stream_event.clear()
