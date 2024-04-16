import socket
import threading
from concurrent.futures import Executor

import pyaudio
import opuslib
import logging

from crypto_manager import CryptoManager
from logging_config import setup_logging
from settings.audio_device import AudioDevice

setup_logging()


class UDPServerStreamer:
    """
    Handles streaming audio from the server to the client over UDP.

    Attributes:
        __client_addr (str): The client's IP address to send audio data to.
        __udp_port (int): The port used for UDP streaming.
        __audio_device (AudioDevice): Audio device used for capturing audio.
        __stop_event (threading.Event): Event to signal the thread to stop.
        __crypto_manager (CryptoManager): Manager for cryptographic operations.
        __executor (Executor): Executor for asynchronous task execution.
    """
    __client_addr: str
    __udp_port: int
    __audio_device: AudioDevice
    __stop_event: threading.Event
    __crypto_manager: CryptoManager
    __executor: Executor

    def __init__(self, client_addr: str, udp_port: int, audio_device: AudioDevice, stop_event: threading.Event,
                 crypto_manager: CryptoManager, executor: Executor):
        """
        Initializes the UDPServerStreamer with specified client address, port, and audio settings.

        Args:
            client_addr (str): Client's address for sending audio data.
            udp_port (int): UDP port to use for audio streaming.
            audio_device (AudioDevice): Configured audio device for capturing audio.
            stop_event (threading.Event): Event to control the shutdown process.
            crypto_manager (CryptoManager): Manager handling encryption and decryption.
            executor (Executor): Executor for managing asynchronous tasks.
        """
        self.__client_addr = client_addr
        self.__udp_port = udp_port
        self.__audio_device = audio_device
        self.__stop_event = stop_event
        self.__crypto_manager = crypto_manager
        self.__executor = executor

    def encode_audio_and_send_to_client(self):
        """
        Captures audio from the selected device, encodes it, encrypts, and sends it to the client over UDP.
        """
        encoder = opuslib.Encoder(self.__audio_device.sample_rate, self.__audio_device.channels,
                                  opuslib.APPLICATION_AUDIO)

        server_socket = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
        stream = self.__audio_device.py_audio.open(format=pyaudio.paInt16,
                                                   channels=self.__audio_device.channels,
                                                   rate=self.__audio_device.sample_rate,
                                                   input=True,
                                                   input_device_index=self.__audio_device.device_index,
                                                   frames_per_buffer=1024)
        logging.info("Start audio streaming...")
        try:
            while not self.__stop_event.is_set():
                data = stream.read(1024, exception_on_overflow=False)

                self.__executor.submit(self.__send_stream_to_client, server_socket, encoder, data)
        except Exception as e:
            logging.error(f"Error in streaming audio: {e}")
        finally:
            stream.stop_stream()
            stream.close()
            server_socket.close()
            self.__audio_device.py_audio.terminate()
            logging.info("Streaming audio stopped.")

    def __send_stream_to_client(self, server_socket, encoder, data):
        """
        Encodes and encrypts audio data, then sends it to the client using UDP.

        Args:
            server_socket (socket.socket): Socket used for sending audio.
            encoder (opuslib.Encoder): Opus encoder for audio data.
            data (bytes): Raw audio data to encode and send.
        """
        encoded_data = self.__crypto_manager.encrypt_and_sign_data(encoder.encode(data, 1024))
        server_socket.sendto(encoded_data, (self.__client_addr, self.__udp_port))
