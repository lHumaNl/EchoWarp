import json
import logging
from dataclasses import dataclass
from typing import Optional

from echowarp.models.default_values_and_options import DefaultValuesAndOptions


@dataclass
class ResponseMessage:
    response_code: int
    response_message: str


class JSONMessage:
    """
    Encapsulates the structure of JSON messages used for communication between the client and server
    in the EchoWarp application.
    Provides methods for encoding and decoding these messages.

    Attributes:
        message (str): The main content of the message.
        response_code (int): An HTTP-like response code indicating the status of the message.
        version (float): The version of the utility compatible with this message format.
    """
    _json_message: dict

    message: str
    response_code: int
    version: str
    failed_connections: int
    reconnect_attempts: Optional[int]

    OK_MESSAGE = ResponseMessage(200, "OK")
    ACCEPTED_MESSAGE = ResponseMessage(202, "Accepted")
    UNAUTHORIZED_MESSAGE = ResponseMessage(401, "Unauthorized")
    FORBIDDEN_MESSAGE = ResponseMessage(403, "Forbidden")
    CONFLICT_MESSAGE = ResponseMessage(409, "Conflict")
    LOCKED_MESSAGE = ResponseMessage(423, "Locked")

    _MESSAGE_KEY = "message"
    _RESPONSE_CODE = "response_code"
    _COMPARABILITY_VERSION_KEY = "comparability_version"
    _FAILED_CONNECTIONS_KEY = "failed_connections"
    _RECONNECT_ATTEMPTS_KEY = "reconnect_attempts"

    def __init__(self, json_bytes: bytes):
        """
                Initializes a new instance of JSONMessage from a byte array containing a JSON string.

                Args:
                    json_bytes (bytes): A byte array containing the JSON-encoded string.

                Raises:
                    Exception: If there is an error decoding the JSON data.
        """
        try:
            self._json_message = json.loads(json_bytes.decode('utf-8'))

            self.message = self._json_message[self._MESSAGE_KEY]
            self.response_code = self._json_message[self._RESPONSE_CODE]
            self.version = self._json_message[self._COMPARABILITY_VERSION_KEY]
            self.failed_connections = self._json_message[self._FAILED_CONNECTIONS_KEY]
            self.reconnect_attempts = self._json_message[self._RECONNECT_ATTEMPTS_KEY]
        except Exception as e:
            logging.error(f"Decode json message error: {e}")
            raise

    @staticmethod
    def encode_message_to_json_bytes(message: str, response_code: int,
                                     failed_connections: Optional[int], reconnect_attempts: Optional[int]) -> bytes:
        """
                Encodes a message with its response code and version into a byte array containing JSON data.

                Args:
                    message (str): The main content of the message.
                    response_code (int): The HTTP-like response code.
                    failed_connections (int): Failed connections.
                    reconnect_attempts (int): Reconnect attempts.

                Returns:
                    bytes: A byte array containing the JSON-encoded message.
        """
        if reconnect_attempts is not None and reconnect_attempts <= 0:
            reconnect_attempts = None

        message = {
            JSONMessage._MESSAGE_KEY: message,
            JSONMessage._RESPONSE_CODE: response_code,
            JSONMessage._COMPARABILITY_VERSION_KEY: DefaultValuesAndOptions.get_util_comparability_version(),
            JSONMessageServer._FAILED_CONNECTIONS_KEY: failed_connections,
            JSONMessageServer._RECONNECT_ATTEMPTS_KEY: reconnect_attempts,
        }

        return json.dumps(message).encode('utf-8')


class JSONMessageServer(JSONMessage):
    """
    Extends JSONMessage to include server-specific configuration settings such as heartbeat attempts,
    encryption status, and integrity control settings.

    Attributes:
        is_encrypt (bool): Indicates if encryption is enabled for the communication.
        is_integrity_control (bool): Indicates if data integrity checks are enabled.
        aes_key_base64 (bytes): AES key used for encryption, provided as a base64-encoded string.
        aes_iv_base64 (bytes): AES initialization vector used for encryption, provided as a base64-encoded string.
    """
    is_encrypt: bool
    is_integrity_control: bool
    aes_key_base64: str
    aes_iv_base64: str

    _CONFIGS_KEY = "config"
    _IS_ENCRYPT_KEY = "is_encrypt"
    _IS_INTEGRITY_CONTROL_KEY = "is_integrity_control"
    _AES_KEY = "aes_key"
    _AES_IV_KEY = "aes_iv"

    def __init__(self, json_bytes: bytes):
        """
        Initializes a new instance of JSONMessageServer from a byte array containing a JSON string with server-specific settings.

        Args:
            json_bytes (bytes): A byte array containing the JSON-encoded string.

        Raises:
            ValueError: If the configuration keys are missing in the JSON data.
            Exception: If there is an error decoding the JSON data.
        """
        super().__init__(json_bytes)

        if self._CONFIGS_KEY in self._json_message:
            try:
                self.is_encrypt = self._json_message[self._CONFIGS_KEY][self._IS_ENCRYPT_KEY]
                self.is_integrity_control = self._json_message[self._CONFIGS_KEY][self._IS_INTEGRITY_CONTROL_KEY]
                self.aes_key_base64 = self._json_message[self._CONFIGS_KEY][self._AES_KEY]
                self.aes_iv_base64 = self._json_message[self._CONFIGS_KEY][self._AES_IV_KEY]
            except Exception as e:
                raise Exception(f"Decode json message error: {e}")
        else:
            raise ValueError(self._CONFIGS_KEY + " not in json config message")

    @staticmethod
    def encode_server_config_to_json_bytes(is_encrypt: bool, is_hash_control: bool,
                                           aes_key_base64: str, aes_iv_base64: str,
                                           failed_connections: int, reconnect_attempts: int) -> bytes:
        """
        Encodes server configuration into a byte array containing JSON data.

        Args:
            is_encrypt (bool): Enable or disable encryption.
            is_hash_control (bool): Enable or disable integrity control.
            aes_key_base64 (str): AES key in Base64 for encryption.
            aes_iv_base64 (str): AES initialization vector in Base64.
            failed_connections (int): Failed connections if client.
            reconnect_attempts (int): Max reconnect attempts on server.

        Returns:
            bytes: A byte array containing the JSON-encoded server configuration.
        """
        if reconnect_attempts <= 0:
            reconnect_attempts = None

        config = {
            JSONMessageServer._MESSAGE_KEY: JSONMessageServer.OK_MESSAGE.response_message,
            JSONMessageServer._RESPONSE_CODE: JSONMessageServer.OK_MESSAGE.response_code,
            JSONMessageServer._COMPARABILITY_VERSION_KEY: DefaultValuesAndOptions.get_util_comparability_version(),
            JSONMessageServer._FAILED_CONNECTIONS_KEY: failed_connections,
            JSONMessageServer._RECONNECT_ATTEMPTS_KEY: reconnect_attempts,

            JSONMessageServer._CONFIGS_KEY: {
                JSONMessageServer._IS_ENCRYPT_KEY: is_encrypt,
                JSONMessageServer._IS_INTEGRITY_CONTROL_KEY: is_hash_control,
                JSONMessageServer._AES_KEY: aes_key_base64,
                JSONMessageServer._AES_IV_KEY: aes_iv_base64,
            }
        }

        return json.dumps(config).encode('utf-8')
