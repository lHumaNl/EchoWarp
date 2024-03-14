import logging
import os
import hashlib
from typing import Tuple, Optional

from cryptography.hazmat.backends import default_backend
from cryptography.hazmat.primitives.asymmetric import rsa, padding
from cryptography.hazmat.primitives import serialization, hashes
from cryptography.hazmat.primitives.ciphers import Cipher, algorithms, modes
from cryptography.hazmat.primitives import padding as sym_padding


class CryptoManager:
    """
    Manages cryptographic operations for EchoWarp, including RSA and AES encryption/decryption.

    This class is designed to handle the cryptographic aspects of the EchoWarp application,
    providing tools for secure communication between the server and the client. It supports
    RSA encryption for initial handshake and key exchange, followed by AES encryption for
    the secure transmission of audio data and heartbeat messages.

    Attributes:
        __is_server (bool): Flag indicating if this instance is used by a server.
        __is_hash_control (bool): Flag indicating if this util used integrity control.
        __private_key (rsa.RSAPrivateKey): The RSA private key for decryption.
        __public_key (rsa.RSAPublicKey): The RSA public key for encryption.
        __aes_key (bytes): The AES key for symmetric encryption of audio data.
        __aes_iv (bytes): The AES initialization vector for symmetric encryption.
        __peer_public_key (Optional[rsa.RSAPublicKey]): The public key of the communication peer.
    """
    __is_server: bool
    __is_hash_control: bool
    __private_key: rsa.RSAPrivateKey
    __public_key: rsa.RSAPublicKey
    __aes_key: Optional[bytes]
    __aes_iv: Optional[bytes]
    __peer_public_key: Optional[rsa.RSAPublicKey]

    def __init__(self, is_server: bool, is_hash_control: bool):
        self.__is_server = is_server
        self.__is_hash_control = is_hash_control

        if self.__is_server:
            self.__generate_aes_key_and_iv()
        else:
            self.__aes_key = None
            self.__aes_iv = None

        self.__private_key, self.__public_key = self.__generate_keys()
        self.__peer_public_key = None

    @staticmethod
    def __generate_keys() -> Tuple[rsa.RSAPrivateKey, rsa.RSAPublicKey]:
        """
        Generates a pair of RSA keys.

        Returns:
            A tuple containing the private key and public key.
        """
        private_key = rsa.generate_private_key(
            public_exponent=65537,
            key_size=2048,
            backend=default_backend()
        )
        public_key = private_key.public_key()

        return private_key, public_key

    def get_serialized_public_key(self):
        return self.__public_key.public_bytes(
            encoding=serialization.Encoding.PEM,
            format=serialization.PublicFormat.SubjectPublicKeyInfo
        )

    def load_peer_public_key(self, pem_public_key):
        """
        Loads and stores the peer's public key from PEM format.

        Args:
            pem_public_key: The peer's public key in PEM format.
        """
        self.__peer_public_key = serialization.load_pem_public_key(
            pem_public_key,
            backend=default_backend()
        )

    def encrypt_rsa_message(self, message):
        """
        Encrypts a message using the peer's public key.

        Args:
            message: The plaintext message to encrypt.

        Returns:
            The encrypted message.
        """
        return self.__peer_public_key.encrypt(
            message,
            padding.OAEP(
                mgf=padding.MGF1(algorithm=hashes.SHA256()),
                algorithm=hashes.SHA256(),
                label=None
            )
        )

    def decrypt_rsa_message(self, encrypted_message):
        """
        Decrypts a message using the private key.

        Args:
            encrypted_message: The encrypted message to decrypt.

        Returns:
            The decrypted plaintext message.
        """

        return self.__private_key.decrypt(encrypted_message,
                                          padding.OAEP(
                                              mgf=padding.MGF1(algorithm=hashes.SHA256()),
                                              algorithm=hashes.SHA256(),
                                              label=None
                                          )
                                          )

    def __generate_aes_key_and_iv(self):
        """
        Generates a new AES key and IV for symmetric encryption.
        """
        self.__aes_key = os.urandom(32)
        self.__aes_iv = os.urandom(16)

    def get_aes_key_and_iv(self):
        """
        Returns the AES key and IV encoded with the peer's public RSA key.
        This method is intended to be used by the server to transmit the AES key and IV securely to the client.
        """

        return self.encrypt_rsa_message(self.__aes_key + self.__aes_iv)

    def load_aes_key_and_iv(self, encrypted_aes_key_iv):
        """
        Loads the AES key and IV by decrypting the message with the private RSA key.
        This method is intended for use by the client to securely receive the AES key and IV from the server.
        """
        decrypted_aes_key_iv = self.decrypt_rsa_message(encrypted_aes_key_iv)
        self.__aes_key = decrypted_aes_key_iv[:32]
        self.__aes_iv = decrypted_aes_key_iv[32:]

    def encrypt_data_aes(self, data):
        """
        Encrypts data using AES.

        Args:
            data: The plaintext data to encrypt.

        Returns:
            The encrypted data.
        """
        padder = sym_padding.PKCS7(128).padder()
        padded_data = padder.update(data) + padder.finalize()

        cipher = Cipher(algorithms.AES(self.__aes_key), modes.CBC(self.__aes_iv))
        encryptor = cipher.encryptor()

        return encryptor.update(padded_data) + encryptor.finalize()

    def decrypt_data_aes(self, encrypted_data):
        """
        Decrypts data using AES.

        Args:
            encrypted_data: The encrypted data to decrypt.

        Returns:
            The decrypted plaintext data.
        """
        cipher = Cipher(algorithms.AES(self.__aes_key), modes.CBC(self.__aes_iv))
        decryptor = cipher.decryptor()
        padded_data = decryptor.update(encrypted_data) + decryptor.finalize()

        unpadder = sym_padding.PKCS7(128).unpadder()

        return unpadder.update(padded_data) + unpadder.finalize()

    @staticmethod
    def __calculate_hash_to_data(data: bytes) -> bytes:
        hasher = hashlib.sha256()
        hasher.update(data)
        data_hash = hasher.digest()
        message = data_hash + data

        return message

    @staticmethod
    def __compare_hash_and_get_data(message: bytes) -> bytes:
        received_hash = message[:32]
        data = message[32:]
        hasher = hashlib.sha256()
        hasher.update(data)
        calculated_hash = hasher.digest()

        if received_hash != calculated_hash:
            raise ValueError("Data integrity check failed")
        else:
            return data
