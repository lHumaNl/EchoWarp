import socket
import threading
import logging


def handle_client(conn, addr, stop_event):
    logging.info(f"Новое подключение: {addr}")
    try:
        while not stop_event.is_set():
            data = conn.recv(1024)
            if not data:
                break
            logging.info(f"Получено от {addr}: {data}")
    except socket.timeout:
        logging.info(f"Timeout соединения с {addr}")
    finally:
        logging.info(f"Закрытие соединения с {addr}")
        conn.close()


def tcp_server(stop_event):
    host = ''
    port = 12346
    server_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    server_socket.bind((host, port))
    server_socket.listen()
    logging.info(f"TCP сервер запущен на порту {port}...")

    try:
        while not stop_event.is_set():
            conn, addr = server_socket.accept()
            conn.settimeout(10)
            client_thread = threading.Thread(target=handle_client, args=(conn, addr, stop_event))
            client_thread.start()
    finally:
        server_socket.close()
        logging.info("TCP сервер остановлен.")
