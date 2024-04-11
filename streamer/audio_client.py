import socket
import threading
from concurrent.futures import ThreadPoolExecutor

import pyaudio
import opuslib
import logging

from logging_config import setup_logging
from settings.audio_device import AudioDevice

setup_logging()


class UDPClientStreamReceiver:
    __server_address: str
    __udp_port: int
    __audio_device: AudioDevice
    __stop_event: threading.Event
    __executor: ThreadPoolExecutor

    def __init__(self, server_address: str, udp_port: int, audio_device: AudioDevice, stop_event: threading.Event):
        self.__server_address = server_address
        self.__udp_port = udp_port
        self.__audio_device = audio_device
        self.__stop_event = stop_event
        self.__executor = ThreadPoolExecutor(max_workers=2)

    def receive_audio_and_decode(self):
        with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as sock:
            sock.bind((self.__server_address, self.__udp_port))
            logging.info("UDP listening started")

            decoder = opuslib.Decoder(self.__audio_device.sample_rate, self.__audio_device.channels)
            stream = self.__audio_device.py_audio.open(
                format=pyaudio.paInt16,
                channels=self.__audio_device.channels,
                rate=self.__audio_device.sample_rate,
                output=True,
                output_device_index=self.__audio_device.device_index
            )

            try:
                while not self.__stop_event.is_set():
                    data, _ = sock.recvfrom(4096)

                    self.__executor.submit(self.__decode_and_play, decoder, data, stream)
            finally:
                stream.stop_stream()
                stream.close()
                logging.info("UDP listening stopped")

    @staticmethod
    def __decode_and_play(decoder, data, stream):
        decoded_data = decoder.decode(data, len(data))
        stream.write(decoded_data)
