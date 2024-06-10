import threading
from concurrent.futures import ThreadPoolExecutor

import pyaudio
import logging

from echowarp.auth_and_heartbeat.transport_client import TransportClient
from echowarp.models.audio_device import AudioDevice
from echowarp.settings import Settings


class ClientStreamReceiver(TransportClient):
    """
        Handles receiving and decoding audio streams on the client side over UDP.

        Attributes:
            _udp_port (int): The port number used for receiving UDP audio streams.
            _audio_device (AudioDevice): Audio device configuration for output.
            _executor (ThreadPoolExecutor): Executor for asynchronous task handling.
    """
    _audio_device: AudioDevice
    _executor: ThreadPoolExecutor

    def __init__(self, settings: Settings, stop_util_event: threading.Event(), stop_stream_event: threading.Event()):
        """
                Initializes a UDP client to receive and play back audio streams.

                Args:
                    settings (Settings): Settings object.
        """
        super().__init__(settings, stop_util_event, stop_stream_event)
        self._audio_device = settings.audio_device
        self._executor = settings.executor

    def receive_audio_and_decode(self):
        """
        Starts the process of receiving audio data from the server over UDP, decrypting and decoding it,
        and then playing it back using the configured audio device.

        This method handles continuous audio streaming until a stop event is triggered.
        """
        if self._audio_device.is_input_device:
            stream = self._audio_device.py_audio.open(
                format=pyaudio.paInt16,
                channels=self._audio_device.channels,
                rate=self._audio_device.sample_rate,
                input=True,
                input_device_index=self._audio_device.device_index
            )
        else:
            stream = self._audio_device.py_audio.open(
                format=pyaudio.paInt16,
                channels=self._audio_device.channels,
                rate=self._audio_device.sample_rate,
                output=True,
                output_device_index=self._audio_device.device_index
            )

        self._print_udp_listener_and_start_stream()
        try:
            while not self._stop_util_event.is_set():
                try:
                    data, _ = self._udp_socket.recvfrom(self._socket_buffer_size)
                    self._executor.submit(self.__decode_and_play, data, stream)
                except Exception:
                    pass

                self._stop_stream_event.wait()
        finally:
            self._executor.shutdown()
            stream.stop_stream()
            stream.close()

            self._audio_device.py_audio.terminate()

            logging.info("UDP listening finished...")

    def __decode_and_play(self, data, stream):
        """
        Decodes the received encrypted audio data and plays it back.

        Args:
            data (bytes): Encrypted audio data.
            stream (pyaudio.Stream): PyAudio stream for audio playback.
        """
        data = self._crypto_manager.decrypt_aes_and_verify_data(data)
        stream.write(data)
