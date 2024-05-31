import base64
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
    Manages cryptographic operations such as RSA and AES encryption/decryption for secure communication in EchoWarp.

    Attributes:
        __is_server (bool): True if this instance is configured for server-side operations.
        is_ssl (bool): Determines if encryption is enabled.
        is_integrity_control (bool): Determines if integrity control via hashing is enabled.
        __private_key (rsa.RSAPrivateKey): The RSA private key for decryption.
        __public_key (rsa.RSAPublicKey): The RSA public key for encryption.
        __aes_key (Optional[bytes]): The AES key for symmetric encryption, used if encryption is enabled.
        __aes_iv (Optional[bytes]): The AES IV for symmetric encryption.
        __peer_public_key (Optional[rsa.RSAPublicKey]): The public key of the communication peer,
        for encrypted communications.
    """
    __is_server: bool
    is_ssl: Optional[bool]
    is_integrity_control: Optional[bool]
    __private_key: rsa.RSAPrivateKey
    __public_key: rsa.RSAPublicKey
    __aes_key: Optional[bytes]
    __aes_iv: Optional[bytes]
    __peer_public_key: rsa.RSAPublicKey

    def __init__(self, is_server: bool, is_integrity_control: bool, is_ssl: bool):
        """
                Initializes a new CryptoManager instance with the specified settings.

                Args:
                    is_server (bool): Indicates if this instance is used by a server.
                    is_integrity_control (bool): Enable or disable integrity control via hashing.
                    is_ssl (bool): Enable or disable encryption.
        """
        self.__is_server = is_server
        self.is_ssl = is_ssl
        self.is_integrity_control = is_integrity_control

        self.__private_key, self.__public_key = self.__generate_and_get_rsa_keys()
        self.__aes_key, self.__aes_iv = self.__generate_and_get_aes_key_and_iv() if is_server else (None, None)

    def load_encryption_config_for_client(self, is_encrypt: bool, is_hash_control: bool):
        if self.__is_server:
            raise ValueError("Load encryption config applied only for client")

        self.is_ssl = is_encrypt
        self.is_integrity_control = is_hash_control

    def encrypt_aes_and_sign_data(self, data: bytes) -> bytes:
        """
        Encrypts and optionally signs data with a hash for integrity.

        Args:
            data (bytes): Data to encrypt and sign.

        Returns:
            bytes: Encrypted (and optionally signed) data.

        Raises:
            ValueError: If data is None.
            Exception: General exceptions during encryption, logged as an error.
        """
        if data is None:
            raise ValueError("Data to encrypt and sign cannot be None")

        if self.is_integrity_control:
            data = self.__calculate_hash_to_data(data)

        if self.is_ssl:
            try:
                data = self.__encrypt_data_aes(data)
            except Exception as e:
                logging.error(f"Failed to encrypt data: {e}")
                raise

        return data

    def decrypt_aes_and_verify_data(self, data: bytes) -> bytes:
        """
        Decrypts and optionally verifies data integrity using a hash.

        Args:
            data (bytes): Data to decrypt and verify.

        Returns:
            bytes: Decrypted data, with integrity optionally verified.

        Raises:
            ValueError: If data is None.
            Exception: General exceptions during decryption, logged as an error.
        """
        if data is None:
            raise ValueError("Data to decrypt and verify cannot be None")

        if self.is_ssl:
            try:
                data = self.__decrypt_data_aes(data)
            except Exception as e:
                logging.error(f"Failed to decrypt data: {e}")
                raise

        if self.is_integrity_control:
            data = self.__compare_hash_and_get_data(data)

        return data

    @staticmethod
    def __generate_and_get_rsa_keys() -> Tuple[rsa.RSAPrivateKey, rsa.RSAPublicKey]:
        """
        Generates a pair of RSA keys.

        Returns:
            A tuple containing the private key and public key.
        """
        private_key = rsa.generate_private_key(
            public_exponent=65537,
            key_size=4096,
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
        Encrypts a message using RSA encryption with the public key of a peer.

        Args:
            message (bytes): The plaintext message to be encrypted.

        Returns:
            bytes: The ciphertext resulting from encrypting the input message with RSA.
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
        Decrypts an RSA encrypted message using the private key of this instance.

        Args:
            encrypted_message (bytes): The encrypted message to be decrypted.

        Returns:
            bytes: The plaintext resulting from decrypting the input message.
        """

        return self.__private_key.decrypt(
            encrypted_message,
            padding.OAEP(
                mgf=padding.MGF1(algorithm=hashes.SHA256()),
                algorithm=hashes.SHA256(),
                label=None
            )
        )

    @staticmethod
    def __generate_and_get_aes_key_and_iv() -> Tuple[bytes, bytes]:
        """
        Generates a new AES key and IV for symmetric encryption.
        """
        aes_key = os.urandom(32)
        aes_iv = os.urandom(16)

        return aes_key, aes_iv

    def get_aes_key_base64(self) -> str:
        """
        Returns the AES key in Base64.
        """

        return base64.b64encode(self.__aes_key).decode('utf-8')

    def get_aes_iv_base64(self) -> str:
        """
        Returns the AES IV in Base64.
        """

        return base64.b64encode(self.__aes_iv).decode('utf-8')

    def load_aes_key_and_iv(self, aes_key_base64: str, aes_iv_base64: str):
        """
        Loads the AES key and IV from peer.
        """
        if self.__is_server:
            logging.error("Only client can load AES key and IV")
            raise ValueError

        self.__aes_key = base64.b64decode(aes_key_base64)
        self.__aes_iv = base64.b64decode(aes_iv_base64)

    def __encrypt_data_aes(self, data):
        """
        Encrypts data using AES.

        Args:
            data: The plaintext data to encrypt.

        Returns:
            The encrypted data.
        """
        padder = sym_padding.PKCS7(128).padder()
        padded_data = padder.update(data) + padder.finalize()

        encryptor = Cipher(algorithms.AES(self.__aes_key), modes.CBC(self.__aes_iv)).encryptor()

        return encryptor.update(padded_data) + encryptor.finalize()

    def __decrypt_data_aes(self, encrypted_data):
        """
        Decrypts data using AES.

        Args:
            encrypted_data: The encrypted data to decrypt.

        Returns:
            The decrypted plaintext data.
        """
        decryptor = Cipher(algorithms.AES(self.__aes_key), modes.CBC(self.__aes_iv)).decryptor()
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
            logging.error("Data integrity check failed")
            raise ValueError
        else:
            return data
