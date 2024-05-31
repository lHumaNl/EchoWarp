import logging
import os
import platform
import subprocess
from dataclasses import dataclass
from typing import List, Optional, Mapping

import chardet
import pyaudio


@dataclass
class Device:
    id: int
    dev: Mapping
    device_name: str
    audio_devices_str: str


class AudioDevice:
    """
    Represents an audio device for recording or playback, handling device selection and configuration.

    Attributes:
        py_audio (pyaudio.PyAudio): Instance of PyAudio used to interface with audio hardware.
        device_name (str): The name of the device.
        device_index (int): The index of the device as used by PyAudio.
        channels (int): Number of audio channels supported by the device.
        sample_rate (int): The sample rate (in Hz) of the device.
    """
    py_audio: pyaudio.PyAudio
    device_name: str
    device_id: Optional[int]
    device_index: int
    channels: int
    sample_rate: int
    ignore_device_encoding_names: bool
    device_encoding_names: str

    def __init__(self, is_input_device: bool, device_id: Optional[int], ignore_device_encoding_names: bool,
                 device_encoding_names: str):
        """
        Initializes an audio device, either for input or output, based on the provided parameters.

        Args:
            is_input_device (bool): Set to True to initialize as an input device, False for output.
            device_id (Optional[int]): Specific device ID to use. If None, the user will select a device interactively.
        """
        self.is_input_device = is_input_device
        self.device_id = device_id
        self.py_audio = pyaudio.PyAudio()

        self.ignore_device_encoding_names = ignore_device_encoding_names
        self.device_encoding_names = device_encoding_names

        if self.device_id is None:
            self.__select_audio_device()
        else:
            self.__select_audio_device_by_device_id()

    def __select_audio_device_by_device_id(self):
        self.device_index = self.device_id
        try:
            dev = self.py_audio.get_device_info_by_index(self.device_index)
        except Exception as e:
            raise Exception(f"Selected invalid device id - {self.device_index}: {e}")

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
        if self.is_input_device:
            device_type = 'Input'
        else:
            device_type = 'Output'

        device_list = self.__get_id_list_of_devices(device_type)
        if len(device_list) == 0:
            raise Exception(f"No connected {device_type} audio devices found!")

        self.__print_list_of_audio_devices(device_type, device_list)
        device_indexes = [device.id for device in device_list]
        device_index = None

        while device_index not in device_indexes:
            try:
                device_index = input()
                device_index = int(device_index)
            except Exception as e:
                logging.error(f'Selected invalid device id: {device_index}{os.linesep}{e}')

        self.device_index = device_index
        self.device_id = device_index
        dev = self.py_audio.get_device_info_by_index(self.device_index)
        self.device_name = self.__decode_string(dev['name'])
        self.channels = dev[f'max{device_type}Channels']
        self.sample_rate = int(dev['defaultSampleRate'])

    @staticmethod
    def __print_list_of_audio_devices(audio_device_type: str, device_list: List[Device]):
        logging.info(
            f"Select id of {audio_device_type} audio devices:{os.linesep}"
            f"{os.linesep.join(device.audio_devices_str for device in device_list)}")

    def __get_id_list_of_devices(self, device_type: str) -> List[Device]:
        device_list = []
        device_count = self.py_audio.get_device_count()

        if device_count == 0:
            raise Exception(f"No connected audio devices found!")

        for i in range(device_count):
            dev = self.py_audio.get_device_info_by_index(i)

            device_name = self.__decode_string(dev['name'])

            if dev[f'max{device_type}Channels'] > 0:
                audio_devices_str = (
                    f"{i}: {device_name}, "
                    f"Channels: {dev[f'max{device_type}Channels']}, "
                    f"Sample rate: {int(dev['defaultSampleRate'])}Hz"
                )
                device_list.append(Device(i, dev, device_name, audio_devices_str))

        if platform.system() == 'Windows':
            device_list = self.__get_data_from_powershell(device_list)

        return device_list

    def __get_data_from_powershell(self, device_list: List[Device]) -> List[Device]:
        command = """
            $OutputEncoding = [Console]::OutputEncoding = [System.Text.Encoding]::UTF8;
            Get-PnpDevice | 
            Where-Object { $_.Class -eq 'AudioEndpoint' -and $_.Status -eq 'OK' } | 
            Select-Object Name | 
            Out-String
            """

        try:
            result = subprocess.run(["powershell", "-Command", command], capture_output=True, text=True,
                                    encoding='utf-8')
            ps_device_names = self.__parse_powershell_stdout(result.stdout)
        except Exception as e:
            logging.warning(f"Failed to get device names from PowerShell: {e}")

            return device_list

        py_audio_device_names = [device.device_name for device in device_list]
        is_ps_device_names_exists = any(ps_device_name in py_audio_device_names for ps_device_name in ps_device_names)

        if not is_ps_device_names_exists:
            logging.warning("Failed to size list of Windows audio devices!")

            return device_list
        else:
            sized_device_list = device_list.copy()
            for id_device in device_list:
                if id_device.device_name not in ps_device_names:
                    sized_device_list.remove(id_device)

            return sized_device_list

    @staticmethod
    def __parse_powershell_stdout(stdout: str) -> List[str]:
        lines = stdout.split('\n')
        device_names = []
        collect_names = False

        for line in lines:
            if '----' in line:
                collect_names = True
                continue

            if collect_names and line.strip():
                device_names.append(line.strip())

        return device_names

    def __decode_string(self, string: str) -> str:
        if self.ignore_device_encoding_names:
            return string

        try:
            return string.encode(self.device_encoding_names).decode('utf-8')
        except (UnicodeEncodeError, UnicodeDecodeError):
            try:
                return string.encode(self.device_encoding_names).decode(
                    chardet.detect(string.encode(self.device_encoding_names))['encoding']
                )
            except (UnicodeEncodeError, UnicodeDecodeError):
                pass

        return string
