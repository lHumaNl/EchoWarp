import logging
import subprocess
from typing import List

import pyaudio


class AudioDevice:
    """
        Represents an audio device for recording or playback.

        Attributes:
            device_name (str): The name of the device.
            device_index (int): The index of the device used by PyAudio.
            channels (int): Number of audio channels.
            sample_rate (int): The sample rate of the device.
    """
    py_audio: pyaudio.PyAudio
    device_name: str
    device_index: int
    channels: int
    sample_rate: int

    def __init__(self, is_input_device: bool, encoding: str):
        """
                Initializes the audio device with the given parameters.

                Args:
                    is_input_device (bool): True if the device is an input device, False otherwise.
                    encoding (str): The encoding used for device names.
        """
        self.__is_input_device = is_input_device
        self.__encoding = encoding
        self.py_audio = pyaudio.PyAudio()
        self.__select_audio_device()

    def __select_audio_device(self):
        """
                Prompts the user to select an audio device from the list of available devices.
        """
        id_list = []
        device_index = None

        if self.__is_input_device:
            device_type = 'Input'
        else:
            device_type = 'Output'

        result_from_powershell = self.get_data_from_powershell()

        logging.info(f"Select id of {device_type} audio device:")

        for i in range(self.py_audio.get_device_count()):
            dev = self.py_audio.get_device_info_by_index(i)

            device_name = self.decode_string(dev['name'])

            if dev[f'max{device_type}Channels'] > 0 and device_name in result_from_powershell:
                id_list.append(i)
                logging.info(
                    f"{i}: {device_name}, Channels: {dev[f'max{device_type}Channels']}, Sample rate: {int(dev['defaultSampleRate'])}Hz")

        while device_index not in id_list:
            if device_index is not None:
                logging.error(f'Selected invalid device id: {device_index}')

            try:
                device_index = input("Select device by id:")
                device_index = int(device_index)
            except Exception:
                pass

        self.device_index = device_index
        dev = self.py_audio.get_device_info_by_index(self.device_index)
        self.device_name = self.decode_string(dev['name'])
        self.channels = dev[f'max{device_type}Channels']
        self.sample_rate = int(dev['defaultSampleRate'])

    def get_data_from_powershell(self) -> List[str]:
        command = """
            $OutputEncoding = [Console]::OutputEncoding = [System.Text.Encoding]::UTF8;
            Get-PnpDevice | 
            Where-Object { $_.Class -eq 'AudioEndpoint' -and $_.Status -eq 'OK' } | 
            Select-Object Name | 
            Out-String
            """
        result = subprocess.run(["powershell", "-Command", command], capture_output=True, text=True)

        device_names = self.parse_powershell_stdout(result.stdout)

        return device_names

    def parse_powershell_stdout(self, stdout: str) -> List[str]:
        decoded_stdout = self.decode_string(stdout)

        lines = decoded_stdout.split('\n')
        device_names = []
        collect_names = False

        for line in lines:
            if '----' in line:
                collect_names = True
                continue
            if collect_names and line.strip():
                device_names.append(line.strip())

        return device_names

    def decode_string(self, string: str) -> str:
        try:
            device_name = string.encode(self.__encoding).decode('utf-8')
        except UnicodeEncodeError:
            device_name = string

        return device_name
