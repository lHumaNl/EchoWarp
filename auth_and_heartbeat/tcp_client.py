import socket
import logging
import threading
import time

from crypto_manager import CryptoManager
from logging_config import setup_logging

setup_logging()


class TCPClient:
    __client_socket: socket
    __server_address: str
    __udp_port: int
    __heartbeat_attempt: int
    __stop_event: threading.Event
    __crypto_manager: CryptoManager

    def __init__(self, server_address: str, udp_port: int, heartbeat_attempt: int, stop_event: threading.Event,
                 crypto_manager: CryptoManager):
        self.__server_address = server_address
        self.__udp_port = udp_port
        self.__heartbeat_attempt = heartbeat_attempt
        self.__stop_event = stop_event
        self.__crypto_manager = crypto_manager
        self.__client_socket = None

    def start_tcp_client(self):
        """
            Initiates a TCP client connection to the server and handles server authentication.

            Raises:
                ConnectionError: If the connection to the server cannot be established or lost.
                ValueError: If authentication with the server fails.
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
        Authenticates with the server by exchanging encrypted messages to verify each other's identity.
        Establishes encryption by exchanging public keys and receiving AES keys.
        """
        # Send client's public key
        self.__client_socket.sendall(self.__crypto_manager.get_serialized_public_key())

        # Receive and load server's public key
        server_public_key_pem = self.__client_socket.recv(1024)
        self.__crypto_manager.load_peer_public_key(server_public_key_pem)

        # Authenticate with server by exchanging specific messages
        encrypted_message_to_server = self.__crypto_manager.encrypt_rsa_message(b"EchoWarpClient")
        self.__client_socket.sendall(encrypted_message_to_server)

        # Receive and decrypt server's authentication message
        encrypted_message_from_server = self.__client_socket.recv(1024)
        message_from_server = self.__crypto_manager.decrypt_rsa_message(encrypted_message_from_server)

        if message_from_server == b"EchoWarpServer":
            logging.info("Server authenticated")
        else:
            raise ValueError("Failed to authenticate server.")

        # Receive encrypted AES key and IV
        encrypted_aes_key_iv = self.__client_socket.recv(1024)
        self.__crypto_manager.load_aes_key_and_iv(encrypted_aes_key_iv)

        logging.info("Authentication and encryption setup completed successfully.")

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
