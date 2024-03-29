import socket
import threading

import pyaudio
import opuslib
import logging

from logging_config import setup_logging
from settings.audio_device import AudioDevice

setup_logging()


class UDPServerStreamer:
    """
        Handles streaming audio from the server to the client over UDP.

        Attributes:
            __client_addr (str): The client's address to send audio data to.
            __udp_port (int): The port used for UDP streaming.
            __audio_device (AudioDevice): Audio device used for capturing audio.
            __stop_event (threading.Event): Event to signal the thread to stop.
    """
    __client_addr: str
    __udp_port: int
    __audio_device: AudioDevice
    __stop_event: threading.Event

    def __init__(self, client_addr: str, udp_port: int, audio_device: AudioDevice, stop_event: threading.Event):
        """
                Initializes the UDPServerStreamer.

                Args:
                    client_addr (str): The client's address.
                    udp_port (int): The UDP port for audio streaming.
                    audio_device (AudioDevice): The audio device to use for capturing audio.
                    stop_event (threading.Event): An event to signal when to stop the stream.
        """
        self.__client_addr = client_addr
        self.__udp_port = udp_port
        self.__audio_device = audio_device
        self.__stop_event = stop_event

    def start_upd_server_streamer(self):
        """
                Starts the UDP server for audio streaming. Captures audio from the specified
                audio device and sends it to the client.

                Raises:
                    Exception: If an error occurs during streaming.
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
                encoded_data = encoder.encode(data, 1024)
                server_socket.sendto(encoded_data, (self.__client_addr, self.__udp_port))
        except Exception as e:
            logging.error(f"Error in streaming audio: {e}")
        finally:
            stream.stop_stream()
            stream.close()
            server_socket.close()
            self.__audio_device.py_audio.terminate()
            logging.info("Streaming audio stopped.")
