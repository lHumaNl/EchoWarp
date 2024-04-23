import socket
import threading
from concurrent.futures import ThreadPoolExecutor

import pyaudio
import logging

from ..auth_and_heartbeat.transport_server import TransportServer
from ..services.crypto_manager import CryptoManager
from ..models.audio_device import AudioDevice


class ServerStreamer(TransportServer):
    """
    Handles streaming audio from the server to the client over UDP.

    Attributes:
        _client_addr (str): The client's IP address to send audio data to.
        _udp_port (int): The port used for UDP streaming.
        _audio_device (AudioDevice): Audio device used for capturing audio.
        _stop_util_event (threading.Event): Event to signal util to stop.
        _stop_stream_event (threading.Event): Event to signal stream to stop.
        _crypto_manager (CryptoManager): Manager for cryptographic operations.
        _executor (Executor): Executor for asynchronous task execution.
    """
    _audio_device: AudioDevice
    _executor: ThreadPoolExecutor

    def __init__(self, udp_port: int, heartbeat_attempt: int, stop_util_event: threading.Event,
                 stop_stream_event: threading.Event, crypto_manager: CryptoManager, audio_device: AudioDevice,
                 executor: ThreadPoolExecutor):
        """
        Initializes the UDPServerStreamer with specified client address, port, and audio settings.

        Args:
            udp_port (int): UDP port to use for audio streaming.
            audio_device (AudioDevice): Configured audio device for capturing audio.
            executor (ThreadPoolExecutor): Executor for managing asynchronous tasks.
        """
        super().__init__(udp_port, heartbeat_attempt, stop_util_event, stop_stream_event, crypto_manager)
        self._audio_device = audio_device
        self._executor = executor

    def encode_audio_and_send_to_client(self):
        """
        Captures audio from the selected device, encodes it, encrypts, and sends it to the client over UDP.
        This method continuously captures and sends audio until a stop event is triggered.
        """
        stream = self._audio_device.py_audio.open(format=pyaudio.paInt16,
                                                  channels=self._audio_device.channels,
                                                  rate=self._audio_device.sample_rate,
                                                  input=True,
                                                  input_device_index=self._audio_device.device_index,
                                                  frames_per_buffer=1024)

        self._print_listener()
        try:
            while not self._stop_util_event.is_set():
                self._stop_stream_event.wait()

                data = stream.read(1024, exception_on_overflow=False)

                try:
                    self._executor.submit(self.__send_stream_to_client, data)
                except socket.error as e:
                    logging.error(f"Failed to send stream: {e}")

        except Exception as e:
            logging.error(f"Error in streaming audio: {e}")
        finally:
            stream.stop_stream()
            stream.close()

            self._audio_device.py_audio.terminate()
            self._cleanup_client_sockets()

            logging.info("Streaming audio stopped.")

    def __send_stream_to_client(self, data):
        """
        Encodes and encrypts audio data, then sends it to the client using UDP.

        Args:
            data (bytes): Raw audio data to encode and send.
        """
        encoded_data = self._crypto_manager.encrypt_aes_and_sign_data(data)
        self._udp_server_socket.sendto(encoded_data, (self._client_addr, self._udp_port))

    def _print_listener(self):
        logging.info(f"Start audio streaming to client on host {self._client_addr}:{self._udp_port}")
