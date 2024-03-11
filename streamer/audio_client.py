import socket
import threading

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

    def __init__(self, server_address: str, udp_port: int, audio_device: AudioDevice, stop_event: threading.Event):
        self.__server_address = server_address
        self.__udp_port = udp_port
        self.__audio_device = audio_device
        self.__stop_event = stop_event

    def start_udp_client_stream_receiver(self):
        decoder = opuslib.Decoder(self.__audio_device.sample_rate, self.__audio_device.channels)

        stream = self.__audio_device.py_audio.open(format=pyaudio.paInt16,
                                                   channels=self.__audio_device.channels,
                                                   rate=self.__audio_device.sample_rate,
                                                   output=True,
                                                   output_device_index=self.__audio_device.device_index)
        logging.info("Start audio stream")
        client_socket = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
        client_socket.bind((self.__server_address, self.__udp_port))
        try:
            while not self.__stop_event.is_set():
                encoded_data, _ = client_socket.recvfrom(4096)
                data = decoder.decode(encoded_data, 4096)
                stream.write(data)
        except Exception as e:
            logging.error(f"Error in receiving audio stream: {e}")
        finally:
            stream.stop_stream()
            stream.close()
            self.__audio_device.py_audio.terminate()
            logging.info("Streaming audio stopped.")
