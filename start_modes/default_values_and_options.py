from start_modes.options_data_creater import OptionsData


class DefaultValuesAndOptions:
    """
    Provides default configuration values and options for various settings within the EchoWarp project.
    This class facilitates the retrieval of default values and options for user interface and setup configurations.

    Attributes:
        __UTIL_VERSION (float): Specifies the current version of the utility.
        __DEFAULT_PORT (int): Default port number used for UDP and TCP communication.
        __DEFAULT_WORKERS_COUNT (int): Default number of worker threads or processes.
        __DEFAULT_HEARTBEAT_ATTEMPT (int): Default number of heartbeat attempts before considering the connection lost.
        __DEFAULT_SERVER_MODE (list): Default server mode option and its boolean value.
        __SERVER_MODE_OPTIONS (list of lists): Available server mode options.
        __DEFAULT_AUDIO_DEVICE_TYPE (list): Default audio device type option and its boolean value.
        __AUDIO_DEVICE_OPTIONS (list of lists): Available audio device options.
        __DEFAULT_SSL (list): Default SSL option and its boolean value.
        __SSL_OPTIONS (list of lists): Available SSL options.
        __DEFAULT_HASH_CONTROL (list): Default hash control option and its boolean value.
        __HASH_CONTROL_OPTIONS (list of lists): Available integrity control options.
    """
    __UTIL_VERSION = 0.1

    __DEFAULT_PORT = 4415
    __DEFAULT_WORKERS_COUNT = 1
    __DEFAULT_HEARTBEAT_ATTEMPT = 5

    SOCKET_BUFFER_SIZE = 6144

    __DEFAULT_SERVER_MODE = ['Server mode', True]
    __SERVER_MODE_OPTIONS = [
        __DEFAULT_SERVER_MODE,
        ['Client mode', False]
    ]

    __DEFAULT_AUDIO_DEVICE_TYPE = ['Input audio device', True]
    __AUDIO_DEVICE_OPTIONS = [
        __DEFAULT_AUDIO_DEVICE_TYPE,
        ['Output audio device', False]
    ]

    __DEFAULT_SSL = ['Disable SSL', False]
    __SSL_OPTIONS = [
        ['Enable SSL', True],
        __DEFAULT_SSL
    ]

    __DEFAULT_HASH_CONTROL = ['Disable integrity control', False]
    __HASH_CONTROL_OPTIONS = [
        ['Enable integrity control', True],
        __DEFAULT_HASH_CONTROL
    ]

    @staticmethod
    def get_util_mods_options_data() -> OptionsData:
        return OptionsData(
            DefaultValuesAndOptions.__DEFAULT_SERVER_MODE,
            DefaultValuesAndOptions.__SERVER_MODE_OPTIONS
        )

    @staticmethod
    def get_audio_device_type_options_data() -> OptionsData:
        return OptionsData(
            DefaultValuesAndOptions.__DEFAULT_AUDIO_DEVICE_TYPE,
            DefaultValuesAndOptions.__AUDIO_DEVICE_OPTIONS
        )

    @staticmethod
    def get_ssl_options_data() -> OptionsData:
        return OptionsData(
            DefaultValuesAndOptions.__DEFAULT_SSL,
            DefaultValuesAndOptions.__SSL_OPTIONS
        )

    @staticmethod
    def get_hash_control_options_data() -> OptionsData:
        return OptionsData(
            DefaultValuesAndOptions.__DEFAULT_HASH_CONTROL,
            DefaultValuesAndOptions.__HASH_CONTROL_OPTIONS
        )

    @staticmethod
    def get_default_port() -> int:
        return DefaultValuesAndOptions.__DEFAULT_PORT

    @staticmethod
    def get_default_heartbeat_attempt() -> int:
        return DefaultValuesAndOptions.__DEFAULT_HEARTBEAT_ATTEMPT

    @staticmethod
    def get_default_workers() -> int:
        return DefaultValuesAndOptions.__DEFAULT_WORKERS_COUNT

    @staticmethod
    def get_util_version() -> float:
        return DefaultValuesAndOptions.__UTIL_VERSION
