import socket
import pyaudio
import logging

from logging_config import setup_logging

setup_logging()


def audio_streaming(udp_port, chosen_device_index, channels, rate, p, stop_event):
    server_socket = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    stream = p.open(format=pyaudio.paInt16,
                    channels=channels,
                    rate=rate,
                    input=True,
                    input_device_index=chosen_device_index,
                    frames_per_buffer=1024)
    logging.info("Начало стриминга аудио...")
    try:
        while not stop_event.is_set():
            data = stream.read(1024, exception_on_overflow=False)
            server_socket.sendto(data, ('<broadcast>', udp_port))
    except Exception as e:
        logging.error(f"Ошибка стриминга аудио: {e}")
    finally:
        stream.stop_stream()
        stream.close()
        server_socket.close()
        p.terminate()
        logging.info("Стриминг аудио завершен.")


def start_server(udp_port, stop_event):
    chosen_device_index, channels, rate, p = select_audio_device(True)
    audio_streaming(udp_port, chosen_device_index, channels, rate, p, stop_event)
