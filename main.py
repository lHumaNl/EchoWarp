import argparse
import threading
from audio_server import start_server
from audio_client import start_client
from logging_config import setup_logging
from tcp_server import tcp_server
from tcp_client import keep_alive

setup_logging()

def parse_arguments():
    parser = argparse.ArgumentParser(description="Аудио стриминговый клиент/сервер")
    parser.add_argument("--config", help="Путь к файлу конфигурации")
    return parser.parse_args()

def read_config(config_path):
    config = configparser.ConfigParser()
    config.read(config_path)
    # Чтение параметров
    return config

def parse_arguments():
    parser = argparse.ArgumentParser(description="Аудио стриминговый клиент/сервер")
    parser.add_argument("--mode", type=str, choices=["server", "client"], help="Режим работы приложения.")
    parser.add_argument("--udp_port", type=int, help="UDP порт для аудио стриминга.")
    parser.add_argument("--server_address", type=str, help="Адрес сервера (для клиента).")
    return parser.parse_args()

def main():
    args = parse_arguments()

    if args.config:
        config = read_config(args.config)
    stop_event = threading.Event()

    if args.mode == "server":
        # Запускаем TCP сервер и аудио стриминг в отдельных потоках
        threading.Thread(target=tcp_server, args=(stop_event,)).start()
        start_server(args.udp_port, stop_event)
    elif args.mode == "client":
        # Запускаем TCP клиент (keep_alive) и аудио приемник в отдельных потоках
        threading.Thread(target=keep_alive, args=(args.server_address, stop_event)).start()
        start_client(args.server_address, args.udp_port, stop_event)

if __name__ == "__main__":
    main()
