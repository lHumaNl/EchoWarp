import locale
import logging
import os
import platform
import subprocess
from typing import List, Optional

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

    def __init__(self, is_input_device: bool, device_id: Optional[int]):
        """
                Initializes the audio device with the given parameters.
        """
        self.__is_input_device = is_input_device
        self.__device_id = device_id
        self.py_audio = pyaudio.PyAudio()

        if self.__device_id is None:
            self.__select_audio_device()
        else:
            self.__select_audio_device_by_device_id()

    def __select_audio_device_by_device_id(self):
        self.device_index = self.__device_id
        dev = self.py_audio.get_device_info_by_index(self.device_index)
        self.device_name = self.__decode_string(dev['name'])
        self.sample_rate = int(dev['defaultSampleRate'])

        if dev['maxInputChannels'] > 0:
            self.channels = dev['maxInputChannels']
        else:
            self.channels = dev['maxOutputChannels']

    def __select_audio_device(self):
        """
                Prompts the user to select an audio device from the list of available devices.
        """
        if self.__is_input_device:
            device_type = 'Input'
        else:
            device_type = 'Output'

        logging.info(f"Select id of {device_type} audio device:")

        id_list = self.__get_id_list_of_devices(device_type)
        device_index = None

        while device_index not in id_list:
            try:
                device_index = input("Select device by id:")
                device_index = int(device_index)
            except Exception as e:
                logging.error(f'Selected invalid device id: {device_index}{os.linesep}{e}')

        self.device_index = device_index
        dev = self.py_audio.get_device_info_by_index(self.device_index)
        self.device_name = self.__decode_string(dev['name'])
        self.channels = dev[f'max{device_type}Channels']
        self.sample_rate = int(dev['defaultSampleRate'])

    def __get_id_list_of_devices(self, device_type: str) -> List[int]:
        id_list = []

        is_windows = platform.system() == 'Windows'
        result_from_powershell = None

        if is_windows:
            result_from_powershell = self.__get_data_from_powershell()

        for i in range(self.py_audio.get_device_count()):
            dev = self.py_audio.get_device_info_by_index(i)

            device_name = self.__decode_string(dev['name'])

            if self.__is_needed_audio_device(dev, device_name, device_type, is_windows, result_from_powershell):
                id_list.append(i)
                logging.info(
                    f"{i}: {device_name}, "
                    f"Channels: {dev[f'max{device_type}Channels']}, "
                    f"Sample rate: {int(dev['defaultSampleRate'])}Hz")

        return id_list

    @staticmethod
    def __is_needed_audio_device(dev, device_name: str, device_type: str, is_windows: bool,
                                 result_from_powershell):
        if is_windows:
            return dev[f'max{device_type}Channels'] > 0 and device_name in result_from_powershell
        else:
            return dev[f'max{device_type}Channels'] > 0

    def __get_data_from_powershell(self) -> List[str]:
        command = """
            $OutputEncoding = [Console]::OutputEncoding = [System.Text.Encoding]::UTF8;
            Get-PnpDevice | 
            Where-Object { $_.Class -eq 'AudioEndpoint' -and $_.Status -eq 'OK' } | 
            Select-Object Name | 
            Out-String
            """

        result = subprocess.run(["powershell", "-Command", command], capture_output=True, text=True, encoding='utf-8')

        device_names = self.__parse_powershell_stdout(result.stdout)

        return device_names

    def __parse_powershell_stdout(self, stdout: str) -> List[str]:
        decoded_stdout = self.__decode_string(stdout)

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

    @staticmethod
    def __decode_string(string: str) -> str:
        try:
            device_name = string.encode(locale.getpreferredencoding()).decode('utf-8')
        except UnicodeEncodeError:
            device_name = string

        return device_name
