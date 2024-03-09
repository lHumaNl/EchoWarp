import logging
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
