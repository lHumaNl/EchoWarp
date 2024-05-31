from concurrent.futures import ThreadPoolExecutor
from typing import Optional

from echowarp.services.crypto_manager import CryptoManager
from echowarp.models.audio_device import AudioDevice


class Settings:
    """
    Configuration settings for the EchoWarp application, which can run in either server or client mode.

    Attributes:
        is_server (bool): True if the instance is configured as a server, otherwise False for a client.
        udp_port (int): The UDP port used for audio streaming.
        server_address (Optional[str]): The address of the server (only relevant in client mode).
        reconnect_attempt (int): The number of allowed missed reconnects before the connection is considered lost.
        audio_device (AudioDevice): The audio device configuration.
        password (Optional[str]): The password for authentication.
        crypto_manager (Optional[CryptoManager]): Manages cryptographic operations, optional for non-secure mode.
        executor (ThreadPoolExecutor): Executor for managing concurrent tasks.
        is_error_log (bool): Indicates if error logging is enabled.
        socket_buffer_size (int): The buffer size for the socket.
    """
    is_server: bool
    udp_port: int
    server_address: Optional[str]
    reconnect_attempt: int
    audio_device: AudioDevice
    password: Optional[str]
    crypto_manager: Optional[CryptoManager]
    executor: ThreadPoolExecutor
    is_error_log: bool
    socket_buffer_size: int

    def __init__(self, is_server: bool, udp_port: int, server_address: Optional[str], reconnect_attempt: int,
                 is_ssl: bool, is_integrity_control: bool, workers: int, audio_device: AudioDevice,
                 password: Optional[str], is_error_log: bool, socket_buffer_size: int):
        """
        Initializes the settings for the EchoWarp application.

        Args:
            is_server (bool): Specifies if the settings are for a server.
            udp_port (int): Port number for UDP communication.
            server_address (Optional[str]): Server address, necessary in client mode.
            reconnect_attempt (int): Number of reconnect attempts before considering a disconnect.
            is_ssl (bool): Enables SSL for secure communication.
            is_integrity_control (bool): Enables integrity control using hashing.
            workers (int): Number of worker threads for handling concurrent operations.
            audio_device (AudioDevice): Configured audio device.
            password (Optional[str]): Password for authentication.
            is_error_log (bool): Indicates if error logging is enabled.
            socket_buffer_size (int): Buffer size for the socket.
        """
        self.is_server = is_server
        self.udp_port = udp_port
        self.server_address = server_address
        self.reconnect_attempt = reconnect_attempt
        self.audio_device = audio_device
        self.password = password
        self.crypto_manager = CryptoManager(self.is_server, is_integrity_control, is_ssl)
        self.executor = ThreadPoolExecutor(max_workers=workers)
        self.is_error_log = is_error_log
        self.socket_buffer_size = socket_buffer_size
