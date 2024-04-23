import base64
import json
import logging

from ..models.default_values_and_options import DefaultValuesAndOptions


class JSONMessage:
    """
    Encapsulates the structure of JSON messages used for communication between the client and server in the EchoWarp application.
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

    AUTH_CLIENT_MESSAGE = "EchoWarpClient"
    AUTH_SERVER_MESSAGE = "EchoWarpServer"
    HEARTBEAT_MESSAGE = "heartbeat"

    _MESSAGE_KEY = "message"
    _RESPONSE_CODE = "response_code"
    _VERSION_KEY = "version"

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
            self.version = self._json_message[self._VERSION_KEY]
        except Exception as e:
            logging.error(f"Decode json message error: {e}")
            raise

    @staticmethod
    def encode_message_to_json_bytes(message: str, response_code: int) -> bytes:
        """
                Encodes a message with its response code and version into a byte array containing JSON data.

                Args:
                    message (str): The main content of the message.
                    response_code (int): The HTTP-like response code.

                Returns:
                    bytes: A byte array containing the JSON-encoded message.
        """
        message = {
            JSONMessage._MESSAGE_KEY: message,
            JSONMessage._RESPONSE_CODE: response_code,
            JSONMessage._VERSION_KEY: DefaultValuesAndOptions.get_util_version(),
        }

        return json.dumps(message).encode('utf-8')


class JSONMessageServer(JSONMessage):
    """
    Extends JSONMessage to include server-specific configuration settings such as heartbeat attempts,
    encryption status, and integrity control settings.

    Attributes:
        heartbeat_attempt (int): The number of allowed missed heartbeats before the server considers the connection lost.
        is_encrypt (bool): Indicates if encryption is enabled for the communication.
        is_integrity_control (bool): Indicates if data integrity checks are enabled.
        aes_key (bytes): AES key used for encryption, provided as a base64-encoded string.
        aes_iv (bytes): AES initialization vector used for encryption, provided as a base64-encoded string.
    """
    heartbeat_attempt: int
    is_encrypt: bool
    is_integrity_control: bool

    _CONFIGS_KEY = "config"
    _HEARTBEAT_ATTEMPT_KEY = "heartbeat_attempt"
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
                self.heartbeat_attempt = self._json_message[self._CONFIGS_KEY][self._HEARTBEAT_ATTEMPT_KEY]
                self.is_encrypt = self._json_message[self._CONFIGS_KEY][self._IS_ENCRYPT_KEY]
                self.is_integrity_control = self._json_message[self._CONFIGS_KEY][self._IS_INTEGRITY_CONTROL_KEY]
                aes_key = self._json_message[self._CONFIGS_KEY][self._AES_KEY]
                aes_iv = self._json_message[self._CONFIGS_KEY][self._AES_IV_KEY]

                self.aes_key = base64.b64decode(aes_key)
                self.aes_iv = base64.b64decode(aes_iv)
            except Exception as e:
                logging.error(f"Decode json message error: {e}")
                raise
        else:
            logging.error(self._CONFIGS_KEY + " not in json config message")
            raise ValueError

    @staticmethod
    def encode_server_config_to_json_bytes(heartbeat_attempt: int, is_encrypt: bool, is_hash_control: bool,
                                           aes_key: bytes, aes_iv: bytes) -> bytes:
        """
        Encodes server configuration into a byte array containing JSON data.

        Args:
            heartbeat_attempt (int): Number of heartbeat attempts before disconnect.
            is_encrypt (bool): Enable or disable encryption.
            is_hash_control (bool): Enable or disable integrity control.
            aes_key (bytes): AES key for encryption.
            aes_iv (bytes): AES initialization vector.

        Returns:
            bytes: A byte array containing the JSON-encoded server configuration.
        """
        encoded_aes_key = base64.b64encode(aes_key).decode('utf-8')
        encoded_aes_iv = base64.b64encode(aes_iv).decode('utf-8')

        config = {
            JSONMessageServer._MESSAGE_KEY: JSONMessageServer.AUTH_SERVER_MESSAGE,
            JSONMessageServer._RESPONSE_CODE: 200,
            JSONMessageServer._VERSION_KEY: DefaultValuesAndOptions.get_util_version(),

            JSONMessageServer._CONFIGS_KEY: {
                JSONMessageServer._HEARTBEAT_ATTEMPT_KEY: heartbeat_attempt,
                JSONMessageServer._IS_ENCRYPT_KEY: is_encrypt,
                JSONMessageServer._IS_INTEGRITY_CONTROL_KEY: is_hash_control,
                JSONMessageServer._AES_KEY: encoded_aes_key,
                JSONMessageServer._AES_IV_KEY: encoded_aes_iv,
            }
        }

        return json.dumps(config).encode('utf-8')
