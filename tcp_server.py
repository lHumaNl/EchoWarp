import logging
import socket
import threading
import time

from logging_config import setup_logging

setup_logging()


def client_handler(conn, addr, stop_event):
    logging.info(f"Установлено соединение с {addr}")
    try:
        # Начальная аутентификация
        conn.sendall(b"EchoWarpServer")
        if conn.recv(1024) == b"EchoWarpClient":
            logging.info(f"Клиент {addr} аутентифицирован")
        else:
            raise ValueError("Ошибка аутентификации")

        # Переходим к "heartbeat"
        while not stop_event.is_set():
            if conn.recv(1024) == b"heartbeat":
                conn.sendall(b"heartbeat")
                time.sleep(5)  # Пауза перед следующим "heartbeat"
            else:
                logging.warning(f"Отсутствие 'heartbeat' от {addr}")
                break
    except Exception as e:
        logging.error(f"Ошибка при работе с клиентом {addr}: {e}")
    finally:
        conn.close()


def start_tcp_server(udp_port, stop_event):
    server_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    server_socket.bind(('0.0.0.0', udp_port))
    server_socket.listen()
    logging.info(f"TCP server start at port: {udp_port}")

    try:
        while not stop_event.is_set():
            conn, addr = server_socket.accept()
            threading.Thread(target=client_handler, args=(conn, addr[0], stop_event)).start()
    except Exception as e:
        logging.error(f"Server error: {e}")
    finally:
        server_socket.close()
        logging.info("TCP server stopped.")
