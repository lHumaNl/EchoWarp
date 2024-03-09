import socket
import pyaudio
import logging
from logging_config import setup_logging

setup_logging()


def select_audio_output_device():
    p = pyaudio.PyAudio()
    print("Доступные устройства вывода:")
    for i in range(p.get_device_count()):
        dev = p.get_device_info_by_index(i)
        if dev['maxOutputChannels'] > 0:
            print(
                f"{i}: {dev['name']} - Каналы: {dev['maxOutputChannels']}, Частота: {int(dev['defaultSampleRate'])}Hz")
    device_index = int(input("Выберите устройство вывода по индексу: "))
    dev = p.get_device_info_by_index(device_index)
    return device_index, dev['maxOutputChannels'], int(dev['defaultSampleRate']), p


def audio_receiving(server_address, udp_port, chosen_device_index, channels, rate, p, stop_event):
    stream = p.open(format=pyaudio.paInt16,
                    channels=channels,
                    rate=rate,
                    output=True,
                    output_device_index=chosen_device_index)
    logging.info("Начало приема аудио...")
    client_socket = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    client_socket.bind((server_address, udp_port))
    try:
        while not stop_event.is_set():
            data, _ = client_socket.recvfrom(2048)
            stream.write(data)
    except Exception as e:
        logging.error(f"Ошибка приема аудио: {e}")
    finally:
        stream.stop_stream()
        stream.close()
        p.terminate()
        logging.info("Прием аудио завершен.")


def start_client(server_address, udp_port, stop_event):
    chosen_device_index, channels, rate, p = select_audio_output_device()
    audio_receiving(server_address, udp_port, chosen_device_index, channels, rate, p, stop_event)
