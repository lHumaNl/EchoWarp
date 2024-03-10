import socket
import threading

import pyaudio
import logging

from logging_config import setup_logging
from settings.audio_device import AudioDevice

setup_logging()


class UDPServerStreamer:
    __client_addr: str
    __udp_port: int
    __audio_device: AudioDevice
    __stop_event: threading.Event

    def __init__(self, client_addr: str, udp_port: int, audio_device: AudioDevice, stop_event: threading.Event):
        self.__client_addr = client_addr
        self.__udp_port = udp_port
        self.__audio_device = audio_device
        self.__stop_event = stop_event

    def start_upd_server_streamer(self):
        server_socket = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
        stream = self.__audio_device.py_audio.open(format=pyaudio.paInt16,
                                                   channels=self.__audio_device.channels,
                                                   rate=self.__audio_device.sample_rate,
                                                   input=True,
                                                   input_device_index=self.__audio_device.device_index,
                                                   frames_per_buffer=1024)
        logging.info("Start audio steaming...")
        try:
            while not self.__stop_event.is_set():
                data = stream.read(1024, exception_on_overflow=False)
                server_socket.sendto(data, (self.__client_addr, self.__udp_port))
        except Exception as e:
            logging.error(f"Error in streaming audio: {e}")
        finally:
            stream.stop_stream()
            stream.close()
            server_socket.close()
            self.__audio_device.py_audio.terminate()
            logging.info("Streaming audio stopped.")
