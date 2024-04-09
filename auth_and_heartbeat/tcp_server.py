import logging
import socket
import threading
import time

from crypto_manager import CryptoManager
from logging_config import setup_logging

setup_logging()


class TCPServer:
    client_addr: str
    __client_connect: socket
    __udp_port: int
    __server_socket: socket
    __heartbeat_attempt: int
    __stop_event: threading.Event
    __crypto_manager: CryptoManager

    def __init__(self, udp_port, heartbeat_attempt: int, stop_event: threading.Event, crypto_manager: CryptoManager):
        self.__udp_port = udp_port
        self.__heartbeat_attempt = heartbeat_attempt
        self.__stop_event = stop_event
        self.__crypto_manager = crypto_manager

        self.__server_socket = None
        self.__client_connect = None

    def start_tcp_server(self):
        """
        Starts the TCP server and waits for a client to connect for authentication and secure communication setup.
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
        Authenticates the client by exchanging encrypted messages to verify each other's identity.
        Establishes encryption by exchanging public keys and setting up AES keys.
        """
        # Send server's public key to client
        self.__client_connect.sendall(self.__crypto_manager.get_serialized_public_key())

        # Receive client's public key
        client_public_key_pem = self.__client_connect.recv(1024)
        self.__crypto_manager.load_peer_public_key(client_public_key_pem)

        # Authenticate client by exchanging specific messages
        encrypted_message_from_client = self.__client_connect.recv(1024)
        message_from_client = self.__crypto_manager.decrypt_rsa_message(encrypted_message_from_client)

        if message_from_client == b"EchoWarpClient":
            logging.info("Client authenticated")

            # Respond with encrypted server authentication message
            encrypted_message_to_client = self.__crypto_manager.encrypt_rsa_message(b"EchoWarpServer")
            self.__client_connect.sendall(encrypted_message_to_client)

            # Send encrypted AES key and IV
            encrypted_aes_key_iv = self.__crypto_manager.get_aes_key_and_iv()
            self.__client_connect.sendall(encrypted_aes_key_iv)

            logging.info("Encryption setup completed successfully.")

            # Start heartbeat thread
            threading.Thread(target=self.__heartbeat, daemon=True).start()
        else:
            raise ValueError("Failed to authenticate client.")

    def __heartbeat(self):
        """
        Continuously receives encrypted and signed heartbeat messages to maintain the connection alive.
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
