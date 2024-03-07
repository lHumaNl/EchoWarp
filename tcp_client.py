import socket
import time
import threading
import logging

def keep_alive(server_address, stop_event):
    port = 12346
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as client_socket:
        try:
            client_socket.connect((server_address, port))
            client_socket.settimeout(10)
            while not stop_event.is_set():
                client_socket.sendall(b'heartbeat')
                time.sleep(5)
        except socket.timeout:
            logging.info("Timeout соединения.")
        finally:
            logging.info("Закрытие TCP соединения.")
