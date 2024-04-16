import socket
import logging
import threading
import time

from crypto_manager import CryptoManager
from logging_config import setup_logging

setup_logging()


class TCPClient:
    """
        Handles TCP client operations for EchoWarp, including server authentication and communication.

        Attributes:
            __server_address (str): IP address or hostname of the server.
            __udp_port (int): Port number for the TCP connection (not UDP).
            __heartbeat_attempt (int): Number of allowed missed heartbeats before disconnecting.
            __stop_event (threading.Event): Event to signal when to stop the client.
            __crypto_manager (CryptoManager): Instance to manage cryptographic operations.
    """
    __client_socket: socket
    __server_address: str
    __udp_port: int
    __heartbeat_attempt: int
    __stop_event: threading.Event
    __crypto_manager: CryptoManager

    def __init__(self, server_address: str, udp_port: int, heartbeat_attempt: int, stop_event: threading.Event,
                 crypto_manager: CryptoManager):
        """
                Initializes the TCPClient with the specified server details and cryptographic manager.

                Args:
                    server_address (str): The server's IP address or hostname.
                    udp_port (int): The TCP port to connect on (despite the name).
                    heartbeat_attempt (int): Max number of missed heartbeats before considering the connection dead.
                    stop_event (threading.Event): Event that signals the client to stop operations.
                    crypto_manager (CryptoManager): Manager handling all cryptographic functions.
        """
        self.__server_address = server_address
        self.__udp_port = udp_port
        self.__heartbeat_attempt = heartbeat_attempt
        self.__stop_event = stop_event
        self.__crypto_manager = crypto_manager
        self.__client_socket = None

    def start_tcp_client(self):
        """
        Establishes the TCP connection to the server and performs authentication using cryptographic methods.

        Raises:
            ConnectionError: If the connection to the server cannot be established or lost unexpectedly.
            ValueError: If the server fails authentication checks.
        """
        self.__client_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)

        try:
            self.__client_socket.connect((self.__server_address, self.__udp_port))
            logging.info(f"TCP connection to {self.__server_address}:{self.__udp_port} established.")

            self.__authenticate_with_server()
        except socket.error as e:
            logging.error(f"Failed to establish connection to {self.__server_address}:{self.__udp_port}: {e}")
            raise ConnectionError(f"Failed to establish connection to {self.__server_address}:{self.__udp_port}")
        except Exception as e:
            logging.error(f"Authentication or encryption setup failed: {e}")
        finally:
            self.__client_socket.close()

    def __authenticate_with_server(self):
        """
        Performs the authentication sequence with the server by exchanging and verifying encrypted data.
        """
        try:
            self.__client_socket.sendall(self.__crypto_manager.get_serialized_public_key())
            server_public_key_pem = self.__client_socket.recv(1024)
            self.__crypto_manager.load_peer_public_key(server_public_key_pem)

            self.__client_socket.sendall(self.__crypto_manager.encrypt_rsa_message(b"EchoWarpClient"))

            encrypted_message_from_server = self.__client_socket.recv(1024)
            message_from_server = self.__crypto_manager.decrypt_rsa_message(encrypted_message_from_server)
            if message_from_server != b"EchoWarpServer":
                raise ValueError("Failed to authenticate server.")

            encrypted_aes_key_iv = self.__client_socket.recv(1024)
            self.__crypto_manager.load_aes_key_and_iv(encrypted_aes_key_iv)

            logging.info("Authentication and encryption setup completed successfully.")
        except Exception as e:
            logging.error(f"Error during authentication: {e}")
            raise

    def __heartbeat(self):
        """
            Continuously sends heartbeat messages to the server to maintain the connection alive.

            Raises:
                RuntimeError: If too many heartbeat messages are missed, indicating a possible connection issue.
        """
        missed_heartbeats = 0

        while not self.__stop_event.is_set() and missed_heartbeats <= self.__heartbeat_attempt:
            try:
                self.__client_socket.sendall(b"heartbeat")
                if self.__client_socket.recv(1024) != b"heartbeat":
                    missed_heartbeats += 1
                    logging.warning(f"Heartbeat miss detected. Miss count: {missed_heartbeats}")
                else:
                    missed_heartbeats = max(0, missed_heartbeats - 1)
                time.sleep(5)
            except socket.error as e:
                logging.error(f"Heartbeat failed due to network error: {e}")
                break

        if missed_heartbeats > self.__heartbeat_attempt:
            logging.error("Too many heartbeat misses, terminating connection.")
            self.__stop_event.set()
