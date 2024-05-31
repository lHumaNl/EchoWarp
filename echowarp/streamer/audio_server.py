import threading
from concurrent.futures import ThreadPoolExecutor

import pyaudio
import logging

from echowarp.auth_and_heartbeat.transport_server import TransportServer
from echowarp.services.crypto_manager import CryptoManager
from echowarp.models.audio_device import AudioDevice
from echowarp.settings import Settings


class ServerStreamer(TransportServer):
    """
    Handles streaming audio from the server to the client over UDP.

    Attributes:
        _client_address (str): The client's IP address to send audio data to.
        _udp_port (int): The port used for UDP streaming.
        _audio_device (AudioDevice): Audio device used for capturing audio.
        _stop_util_event (threading.Event): Event to signal util to stop.
        _stop_stream_event (threading.Event): Event to signal stream to stop.
        _crypto_manager (CryptoManager): Manager for cryptographic operations.
        _executor (Executor): Executor for asynchronous task execution.
    """
    _audio_device: AudioDevice
    _executor: ThreadPoolExecutor

    def __init__(self, settings: Settings, stop_util_event: threading.Event(), stop_stream_event: threading.Event()):
        """
        Initializes the UDPServerStreamer with specified client address, port, and audio settings.

        Args:
            settings (Settings): Settings object.
        """
        super().__init__(settings, stop_util_event, stop_stream_event)
        self._audio_device = settings.audio_device
        self._executor = settings.executor

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

        self._print_udp_listener_and_start_stream()
        try:
            while not self._stop_util_event.is_set():
                try:
                    data = stream.read(1024, exception_on_overflow=False)
                    self._executor.submit(self.__send_stream_to_client, data)
                except Exception:
                    pass

                self._stop_stream_event.wait()
        finally:
            self._executor.shutdown()
            stream.stop_stream()
            stream.close()

            self._audio_device.py_audio.terminate()

            logging.info("UDP streaming finished...")

    def __send_stream_to_client(self, data):
        """
        Encodes and encrypts audio data, then sends it to the client using UDP.

        Args:
            data (bytes): Raw audio data to encode and send.
        """
        encoded_data = self._crypto_manager.encrypt_aes_and_sign_data(data)
        self._udp_socket.sendto(encoded_data, (self._client_address, self._udp_port))
