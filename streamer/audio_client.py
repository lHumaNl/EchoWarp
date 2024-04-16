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


class UDPClientStreamReceiver:
    """
        Handles receiving and decoding audio streams on the client side over UDP.

        Attributes:
            __server_address (str): The server's IP address from which to receive the audio.
            __udp_port (int): The port number used for receiving UDP audio streams.
            __audio_device (AudioDevice): Audio device configuration for output.
            __stop_event (threading.Event): Event to signal the termination of audio receiving.
            __crypto_manager (CryptoManager): Cryptographic manager for secure communication.
            __executor (Executor): Executor for asynchronous task handling.
    """
    __server_address: str
    __udp_port: int
    __audio_device: AudioDevice
    __stop_event: threading.Event
    __crypto_manager: CryptoManager
    __executor: Executor

    def __init__(self, server_address: str, udp_port: int, audio_device: AudioDevice, stop_event: threading.Event,
                 crypto_manager: CryptoManager, executor: Executor):
        """
                Initializes a UDP client to receive and play back audio streams.

                Args:
                    server_address (str): The IP address of the audio source server.
                    udp_port (int): The UDP port to listen for incoming audio data.
                    audio_device (AudioDevice): Configured audio device for output.
                    stop_event (threading.Event): Event to control the shutdown process.
                    crypto_manager (CryptoManager): Manager for handling encryption and decryption.
                    executor (Executor): Executor for running tasks asynchronously.
        """
        self.__server_address = server_address
        self.__udp_port = udp_port
        self.__audio_device = audio_device
        self.__stop_event = stop_event
        self.__crypto_manager = crypto_manager
        self.__executor = executor

    def receive_audio_and_decode(self):
        """
            Starts receiving and decoding the audio stream from the server using UDP.
        """
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

    def __decode_and_play(self, decoder, data, stream):
        """
        Decodes the received encrypted audio data and plays it back.

        Args:
            decoder (opuslib.Decoder): Decoder instance for Opus audio format.
            data (bytes): Encrypted audio data.
            stream (pyaudio.Stream): PyAudio stream for audio playback.
        """
        data = self.__crypto_manager.decrypt_and_verify_data(data)

        decoded_data = decoder.decode(data, len(data))
        stream.write(decoded_data)
