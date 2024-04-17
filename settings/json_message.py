import json
import logging
from logging_config import setup_logging

from start_modes.default_values_and_options import DefaultValuesAndOptions

setup_logging()


class JSONMessage:
    """
        Represents a JSON message used for communication between client and server. This class encapsulates
        the message structure and provides methods for encoding and decoding messages.

        Attributes:
            message (str): The main content of the message.
            response_code (int): The HTTP-like response code indicating the status of the message.
            version (float): The version of the utility that the message is compatible with.
    """
    __json_message: dict

    message: str
    response_code: int
    version: float

    AUTH_CLIENT_MESSAGE = "EchoWarpClient"
    AUTH_SERVER_MESSAGE = "EchoWarpServer"
    HEARTBEAT_MESSAGE = "heartbeat"

    __MESSAGE_KEY = "message"
    __RESPONSE_CODE = "response_code"
    __VERSION_KEY = "version"

    def __init__(self, json_bytes: bytes):
        """
                Initializes a new instance of JSONMessage from a byte array containing a JSON string.

                Args:
                    json_bytes (bytes): A byte array containing the JSON-encoded string.

                Raises:
                    Exception: If there is an error decoding the JSON data.
        """
        try:
            self.__json_message = json.loads(json_bytes.decode('utf-8'))

            self.message = self.__json_message[self.__MESSAGE_KEY]
            self.response_code = self.__json_message[self.__RESPONSE_CODE]
            self.version = self.__json_message[self.__VERSION_KEY]
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
            JSONMessage.__MESSAGE_KEY: message,
            JSONMessage.__RESPONSE_CODE: response_code,
            JSONMessage.__VERSION_KEY: DefaultValuesAndOptions.get_util_version(),
        }

        return json.dumps(message).encode('utf-8')


class JSONMessageServer(JSONMessage):
    """
    Represents a server-specific JSON message that includes configuration settings in addition to the basic message structure.

    Attributes:
        heartbeat_attempt (int): Number of allowed missed heartbeats before the server considers the connection lost.
        is_encrypt (bool): Indicates whether encryption is enabled.
        is_integrity_control (bool): Indicates whether integrity control is enabled.
        aes_key (bytes): The AES key used for encryption.
        aes_iv (bytes): The AES initialization vector.
    """
    heartbeat_attempt: int
    is_encrypt: bool
    is_integrity_control: bool

    __CONFIGS_KEY = "config"
    __HEARTBEAT_ATTEMPT_KEY = "heartbeat_attempt"
    __IS_ENCRYPT_KEY = "is_encrypt"
    __IS_INTEGRITY_CONTROL_KEY = "is_integrity_control"
    __AES_KEY = "aes_key"
    __AES_IV_KEY = "aes_iv"

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

        if self.__CONFIGS_KEY in self.__json_message:
            try:
                self.heartbeat_attempt = self.__json_message[self.__CONFIGS_KEY][self.__HEARTBEAT_ATTEMPT_KEY]
                self.is_encrypt = self.__json_message[self.__CONFIGS_KEY][self.__IS_ENCRYPT_KEY]
                self.is_integrity_control = self.__json_message[self.__CONFIGS_KEY][self.__IS_INTEGRITY_CONTROL_KEY]
                self.aes_key = self.__json_message[self.__CONFIGS_KEY][self.__AES_KEY]
                self.aes_iv = self.__json_message[self.__CONFIGS_KEY][self.__AES_IV_KEY]
            except Exception as e:
                logging.error(f"Decode json message error: {e}")
                raise
        else:
            logging.error(self.__CONFIGS_KEY + " not in json config message")
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
        config = {
            JSONMessageServer.__MESSAGE_KEY: JSONMessageServer.AUTH_SERVER_MESSAGE,
            JSONMessageServer.__RESPONSE_CODE: 200,
            JSONMessageServer.__VERSION_KEY: DefaultValuesAndOptions.get_util_version(),

            JSONMessageServer.__CONFIGS_KEY: {
                JSONMessageServer.__HEARTBEAT_ATTEMPT_KEY: heartbeat_attempt,
                JSONMessageServer.__IS_ENCRYPT_KEY: is_encrypt,
                JSONMessageServer.__IS_INTEGRITY_CONTROL_KEY: is_hash_control,
                JSONMessageServer.__AES_KEY: aes_key,
                JSONMessageServer.__AES_IV_KEY: aes_iv,
            }
        }

        return json.dumps(config).encode('utf-8')
