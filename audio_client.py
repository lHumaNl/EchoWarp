import socket
import pyaudio
import logging

from logging_config import setup_logging

setup_logging()


def audio_receiving(server_address, udp_port, chosen_device_index, channels, rate, p, stop_event):
    stream = p.open(format=pyaudio.paInt16,
                    channels=channels,
                    rate=rate,
                    output=True,
                    output_device_index=chosen_device_index)
    logging.info("Start audio steam")
    client_socket = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    client_socket.bind((server_address, udp_port))
    try:
        while not stop_event.is_set():
            data, _ = client_socket.recvfrom(2048)
            stream.write(data)
    except Exception as e:
        logging.error(f"Error to get audio stream: {e}")
    finally:
        stream.stop_stream()
        stream.close()
        p.terminate()
        logging.info("Streaming audio stopped.")


def start_client(server_address, udp_port, stop_event):
    chosen_device_index, channels, rate, p = select_audio_device(False)
    audio_receiving(server_address, udp_port, chosen_device_index, channels, rate, p, stop_event)
