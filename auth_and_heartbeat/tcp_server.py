import logging
import socket
import threading
import time

from crypto_manager import CryptoManager
from logging_config import setup_logging

setup_logging()


class TCPServer:
    """
        Handles the server-side TCP operations for EchoWarp, including client authentication and secure communication setup.

        Attributes:
            client_addr (str): IP address of the connected client.
            __client_connect (socket.socket): Socket object for the connected client.
            __udp_port (int): TCP port on which the server listens (used for initial TCP handshake).
            __server_socket (socket.socket): Server socket to accept connections.
            __heartbeat_attempt (int): Number of heartbeat misses allowed before disconnecting.
            __stop_event (threading.Event): Event to signal when to stop the server.
            __crypto_manager (CryptoManager): Manager for cryptographic operations.
    """
    client_addr: str
    __client_connect: socket
    __udp_port: int
    __server_socket: socket
    __heartbeat_attempt: int
    __stop_event: threading.Event
    __crypto_manager: CryptoManager

    def __init__(self, udp_port, heartbeat_attempt: int, stop_event: threading.Event, crypto_manager: CryptoManager):
        """
                Initializes the TCPServer with the specified port and cryptographic manager.

                Args:
                    udp_port (int): The TCP port to listen on for incoming connections.
                    heartbeat_attempt (int): Number of allowed missed heartbeats before considering the connection lost.
                    stop_event (threading.Event): Event that signals the server to stop operations.
                    crypto_manager (CryptoManager): Manager handling all cryptographic functions.
        """
        self.__udp_port = udp_port
        self.__heartbeat_attempt = heartbeat_attempt
        self.__stop_event = stop_event
        self.__crypto_manager = crypto_manager

        self.__server_socket = None
        self.__client_connect = None

    def start_tcp_server(self):
        """
        Starts the TCP server, listens for incoming connections, and handles client authentication and setup.
        """
        self.__server_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self.__server_socket.bind(('0.0.0.0', self.__udp_port))
        self.__server_socket.listen(1)
        logging.info(f'TCP server started on port "{self.__udp_port}" awaiting client connection')

        try:
            self.__client_connect, client_addr = self.__server_socket.accept()
            self.client_addr = client_addr[0]
            logging.info(f"Client connected from {self.client_addr}")

            self.__authenticate_client()
        except Exception as e:
            logging.error(f"Server encountered an error: {e}")
        finally:
            if self.__client_connect:
                self.__client_connect.close()
            self.__server_socket.close()

    def __authenticate_client(self):
        """
        Handles the authentication sequence with the client by exchanging encrypted messages.
        Establishes encryption settings by exchanging public keys and AES keys.
        """
        try:
            self.__client_connect.sendall(self.__crypto_manager.get_serialized_public_key())

            client_public_key_pem = self.__client_connect.recv(1024)
            self.__crypto_manager.load_peer_public_key(client_public_key_pem)

            encrypted_message_from_client = self.__client_connect.recv(1024)
            message_from_client = self.__crypto_manager.decrypt_rsa_message(encrypted_message_from_client)
            if message_from_client == b"EchoWarpClient":
                logging.info("Client authenticated")

                self.__client_connect.sendall(self.__crypto_manager.encrypt_rsa_message(b"EchoWarpServer"))
                self.__client_connect.sendall(self.__crypto_manager.get_aes_key_and_iv())

                logging.info("Encryption setup completed successfully.")

                threading.Thread(target=self.__heartbeat, daemon=True).start()
            else:
                raise ValueError("Failed to authenticate client.")
        except Exception as e:
            logging.error(f"Authentication error: {e}")
            raise

    def __heartbeat(self):
        """
        Handles the heartbeat mechanism to ensure the connection remains alive and stable.
        """
        while not self.__stop_event.is_set():
            try:
                encrypted_message = self.__client_connect.recv(1024)
                message = self.__crypto_manager.decrypt_and_verify_data(encrypted_message)

                if message == b"heartbeat":
                    response = self.__crypto_manager.encrypt_and_sign_data(message)
                    self.__client_connect.sendall(response)
                else:
                    logging.warning("Heartbeat verification failed.")
                time.sleep(5)
            except Exception as e:
                logging.error(f"Heartbeat failed: {e}")
                break
