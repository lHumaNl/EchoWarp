from typing import Optional, List


class Settings:
    is_server: bool
    udp_port: int
    server_addr: Optional[int]

    __CLIENT_MODE = [1, 'client']
    __SERVER_MODE = [2, 'server']
    __DEFAULT_PORT = 6532

    def __init__(self, is_server: bool, udp_port: int, server_addr: Optional[int]):
        self.is_server = is_server
        self.udp_port = udp_port
        self.server_addr = server_addr

    @staticmethod
    def get_client_value() -> List:
        return Settings.__CLIENT_MODE

    @staticmethod
    def get_server_value() -> List:
        return Settings.__SERVER_MODE

    @staticmethod
    def get_default_port():
        return Settings.__DEFAULT_PORT
