import socket
import threading
from concurrent.futures import ThreadPoolExecutor

import pyaudio
import logging

from echowarp.auth_and_heartbeat.transport_client import TransportClient
from echowarp.models.audio_device import AudioDevice
from echowarp.models.default_values_and_options import DefaultValuesAndOptions
from echowarp.services.crypto_manager import CryptoManager


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

    def __init__(self, server_address: str, udp_port: int, stop_util_event: threading.Event,
                 stop_stream_event: threading.Event, crypto_manager: CryptoManager, audio_device: AudioDevice,
                 executor: ThreadPoolExecutor):
        """
                Initializes a UDP client to receive and play back audio streams.

                Args:
                    server_address (str): The IP address of the audio source server.
                    udp_port (int): The UDP port to listen for incoming audio data.
                    audio_device (AudioDevice): Configured audio device for output.
                    executor (Executor): Executor for running tasks asynchronously.
        """
        super().__init__(server_address, udp_port, stop_util_event, stop_stream_event, crypto_manager)
        self._audio_device = audio_device
        self._executor = executor

    def receive_audio_and_decode(self):
        """
        Starts the process of receiving audio data from the server over UDP, decrypting and decoding it,
        and then playing it back using the configured audio device.

        This method handles continuous audio streaming until a stop event is triggered.
        """
        stream = self._audio_device.py_audio.open(
            format=pyaudio.paInt16,
            channels=self._audio_device.channels,
            rate=self._audio_device.sample_rate,
            output=True,
            output_device_index=self._audio_device.device_index
        )

        self._print_listener()
        try:
            while not self._stop_util_event.is_set():
                self._stop_stream_event.wait()

                try:
                    data, _ = self._client_udp_socket.recvfrom(DefaultValuesAndOptions.SOCKET_BUFFER_SIZE)
                except socket.timeout as e:
                    logging.error(f"Timeout to receive stream: {e}")
                    continue
                except socket.error as e:
                    logging.error(f"Failed to receive stream: {e}")
                    continue

                self._executor.submit(self.__decode_and_play, data, stream)
        except Exception as e:
            logging.error(f"Error in streaming audio: {e}")
        finally:
            stream.stop_stream()
            stream.close()

            self._audio_device.py_audio.terminate()
            self._cleanup_client_sockets()

            logging.info("UDP listening stopped")

    def __decode_and_play(self, data, stream):
        """
        Decodes the received encrypted audio data and plays it back.

        Args:
            data (bytes): Encrypted audio data.
            stream (pyaudio.Stream): PyAudio stream for audio playback.
        """
        data = self._crypto_manager.decrypt_aes_and_verify_data(data)
        stream.write(data)

    def _print_listener(self):
        logging.info(f'UDP listening started from host {self._server_address}:{self._udp_port}')
