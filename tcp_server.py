import logging
import socket
import threading
import time

from logging_config import setup_logging

setup_logging()


class TCPServer:
    def __init__(self):
        self.__start_tcp_server()
    def __start_tcp_server(self, udp_port, heartbeat_attempt: int, stop_event: threading.Event()):
        server_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        server_socket.bind(('0.0.0.0', udp_port))
        server_socket.listen()
        logging.info(f"TCP server started at port: {udp_port}")

        try:
            conn, addr = server_socket.accept()
        except Exception as e:
            server_socket.close()
            logging.error(f"Server error: {e}")
            raise Exception

        try:
            self.__client_handler(conn, addr[0], heartbeat_attempt, stop_event)
        except Exception as e:
            server_socket.close()
            logging.error(f"Server error: {e}")
            raise Exception

        return addr

    def __heartbeat(self, conn, addr, heartbeat_attempt: int, stop_event: threading.Event()):
        heartbeat_fails = 0

        while not stop_event.is_set():
            if heartbeat_fails < 0:
                heartbeat_fails = 0

            if heartbeat_fails > heartbeat_attempt:
                raise ValueError(f"Heartbeat fails is too many: {heartbeat_fails}")

            if conn.recv(1024) == b"heartbeat":
                conn.sendall(b"heartbeat")
                if heartbeat_fails >= 1:
                    heartbeat_fails -= 1

                time.sleep(5)
            else:
                heartbeat_fails += 1
                logging.warning(f"Missed 'heartbeat' from {addr}. Missed 'heartbeat': {heartbeat_fails}")

    def __client_handler(self, conn, addr, heartbeat_attempt: int, stop_event: threading.Event()):
        logging.info(f"Success connect to {addr}")
        conn.sendall(b"EchoWarpServer")

        if conn.recv(1024) == b"EchoWarpClient":
            logging.info(f"Client {addr} authenticated")
        else:
            raise ValueError("Authentication error")

        threading.Thread(target=self.__heartbeat, args=(conn, addr, heartbeat_attempt, stop_event)).start()
