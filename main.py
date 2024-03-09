import sys
import threading
import logging

import args_mode
import interactive_mode
from logging_config import setup_logging
from settings import Settings

from tcp_server import client_handler
from tcp_client import start_tcp_client
from audio_server import start_server
from audio_client import start_client

setup_logging()


def main():
    if len(sys.argv) == 1:
        settings: Settings = interactive_mode.get_settings_interactive()
    else:
        settings: Settings = args_mode.get_settings_by_args()

    stop_event = threading.Event()
    tcp_thread = None
    tcp_client_thread = None

    if settings.is_server:
        logging.info("Start EchoWarp in server mode.")
        # Запускаем TCP сервер для аутентификации клиента
        tcp_thread = threading.Thread(target=client_handler, args=(stop_event,))
        tcp_thread.start()
        # Запуск сервера для стриминга аудио после установления TCP соединения
        start_server(settings.udp_port, stop_event)

    else:
        logging.info("Start EchoWarp in client mode.")
        # Установление TCP соединения с сервером для аутентификации
        tcp_client_thread = threading.Thread(target=start_tcp_client, args=(settings.server_addr, stop_event))
        tcp_client_thread.start()
        # Запуск клиента для приема аудио после установления TCP соединения
        start_client(settings.server_addr, settings.udp_port, stop_event)

    try:
        input("Нажмите Enter для выхода...\n")
    finally:
        stop_event.set()
        if settings.is_server:
            tcp_thread.join()
        else:
            tcp_client_thread.join()


if __name__ == "__main__":
    main()
