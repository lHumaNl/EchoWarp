import socket
import logging
import time

from logging_config import setup_logging

setup_logging()


def start_tcp_client(server_address: str, udp_port: int, stop_event):
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as client_socket:
        client_socket.connect((server_address, udp_port))
        logging.info("TCP соединение установлено")

        # Начальная аутентификация
        if client_socket.recv(1024) == b"EchoWarpServer":
            client_socket.sendall(b"EchoWarpClient")
            logging.info("Сервер аутентифицирован")
        else:
            raise ValueError("Ошибка аутентификации")

        # Переходим к "heartbeat"
        while not stop_event.is_set():
            client_socket.sendall(b"heartbeat")
            if client_socket.recv(1024) == b"heartbeat":
                time.sleep(5)
            else:
                logging.warning("Отсутствие 'heartbeat' от сервера")
                break
