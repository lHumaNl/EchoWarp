from typing import Tuple

from cryptography.hazmat.backends import default_backend
from cryptography.hazmat.primitives.asymmetric import rsa, padding
from cryptography.hazmat.primitives import serialization, hashes


class CryptoManager:
    """
    A class to manage cryptographic operations for EchoWarp including key generation,
    encryption, and decryption using RSA.
    """
    __private_key: rsa.RSAPrivateKey
    __public_key: rsa.RSAPublicKey
    peer_public_key: rsa.RSAPublicKey

    def __init__(self):
        self.__private_key, self.__public_key = self.__generate_keys()

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
        self.peer_public_key = serialization.load_pem_public_key(
            pem_public_key,
            backend=default_backend()
        )

    def encrypt_message(self, message):
        """
        Encrypts a message using the peer's public key.

        Args:
            message: The plaintext message to encrypt.

        Returns:
            The encrypted message.
        """
        return self.peer_public_key.encrypt(
            message,
            padding.OAEP(
                mgf=padding.MGF1(algorithm=hashes.SHA256()),
                algorithm=hashes.SHA256(),
                label=None
            )
        )

    def decrypt_message(self, encrypted_message):
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
