from concurrent.futures import ThreadPoolExecutor
from typing import Optional

from .services.crypto_manager import CryptoManager
from .models.audio_device import AudioDevice


class Settings:
    """
    Configuration settings for the EchoWarp application, which can run in either server or client mode.

    Attributes:
        is_server (bool): True if the instance is configured as a server, otherwise False for a client.
        udp_port (int): The UDP port used for audio streaming.
        server_addr (Optional[str]): The address of the server (only relevant in client mode).
        heartbeat_attempt (int): The number of allowed missed heartbeats before the connection is considered lost.
        audio_device (AudioDevice): The audio device configuration.
        crypto_manager (Optional[CryptoManager]): Manages cryptographic operations, optional for non-secure mode.
        executor (ThreadPoolExecutor): Executor for managing concurrent tasks.
    """
    is_server: bool
    udp_port: int
    server_addr: Optional[str]
    heartbeat_attempt: int
    audio_device: AudioDevice
    crypto_manager: Optional[CryptoManager]
    executor: ThreadPoolExecutor

    def __init__(self, is_server_mode: bool, udp_port: int, server_addr: Optional[str], heartbeat_attempt: int,
                 is_ssl: bool, is_hash_control: bool, workers: int, audio_device: AudioDevice):
        """
        Initializes the settings for the EchoWarp application.

        Args:
            is_server_mode (bool): Specifies if the settings are for a server.
            udp_port (int): Port number for UDP communication.
            server_addr (Optional[str]): Server address, necessary in client mode.
            heartbeat_attempt (int): Number of heartbeat attempts before considering a disconnect.
            is_ssl (bool): Enables SSL for secure communication.
            is_hash_control (bool): Enables integrity control using hashing.
            workers (int): Number of worker threads for handling concurrent operations.
            audio_device (AudioDevice): Configured audio device.
        """
        self.is_server = is_server_mode
        self.udp_port = udp_port
        self.server_addr = server_addr
        self.heartbeat_attempt = heartbeat_attempt
        self.audio_device = audio_device
        self.crypto_manager = CryptoManager(self.is_server, is_hash_control, is_ssl)
        self.executor = ThreadPoolExecutor(max_workers=workers)
