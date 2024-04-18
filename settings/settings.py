from concurrent.futures import ThreadPoolExecutor
from typing import Optional

from crypto_manager import CryptoManager
from settings.audio_device import AudioDevice


class Settings:
    is_server: bool
    udp_port: int
    server_addr: Optional[str]
    heartbeat_attempt: int
    audio_device: AudioDevice
    crypto_manager: Optional[CryptoManager]
    executor: ThreadPoolExecutor

    def __init__(self, is_server_mode: bool, udp_port: int, server_addr: Optional[str], heartbeat_attempt: int,
                 is_ssl: bool, is_hash_control: bool, workers: int, audio_device: AudioDevice):
        self.is_server = is_server_mode
        self.udp_port = udp_port
        self.server_addr = server_addr
        self.heartbeat_attempt = heartbeat_attempt
        self.audio_device = audio_device
        self.crypto_manager = CryptoManager(self.is_server, is_hash_control, is_ssl)
        self.executor = ThreadPoolExecutor(max_workers=workers)
