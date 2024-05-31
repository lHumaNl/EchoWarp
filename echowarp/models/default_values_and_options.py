import locale

from echowarp.models.options_data_creater import OptionsData
import version


class DefaultValuesAndOptions:
    """
    Provides default configuration values and options for various settings within the EchoWarp project.
    This class facilitates the retrieval of default values and options for user interface and setup configurations.

    Attributes:
        __DEFAULT_PORT (int): Default port number used for UDP and TCP communication.
        __DEFAULT_WORKERS_COUNT (int): Default number of worker threads or processes.
        __DEFAULT_RECONNECT_ATTEMPT (int): Default number of heartbeat attempts before considering the connection lost.
        __DEFAULT_SERVER_MODE (list): Default server mode option and its boolean value.
        __SERVER_MODE_OPTIONS (list of lists): Available server mode options.
        __DEFAULT_AUDIO_DEVICE_TYPE (list): Default audio device type option and its boolean value.
        __AUDIO_DEVICE_OPTIONS (list of lists): Available audio device options.
        __DEFAULT_SSL (list): Default SSL option and its boolean value.
        __SSL_OPTIONS (list of lists): Available SSL options.
        __DEFAULT_HASH_CONTROL (list): Default hash control option and its boolean value.
        __HASH_CONTROL_OPTIONS (list of lists): Available integrity control options.
    """
    __DEFAULT_PORT = 4415
    __DEFAULT_WORKERS_COUNT = 1
    __DEFAULT_RECONNECT_ATTEMPT = 5
    __DEFAULT_BUFFER_SIZE = 6144
    __DEFAULT_TIMEOUT = 5
    __DEFAULT_HEARTBEAT_DELAY = 2

    CONFIG_TITLE = 'echowarp_conf'
    BAN_LIST_FILE = 'echowarp_ban_list.txt'

    __DEFAULT_SERVER_MODE = ['Server mode', True]
    __SERVER_MODE_OPTIONS = [
        __DEFAULT_SERVER_MODE,
        ['Client mode', False]
    ]

    __DEFAULT_SOCKET_BUFFER_SIZE = [f'{__DEFAULT_BUFFER_SIZE}', __DEFAULT_BUFFER_SIZE]
    __SOCKET_BUFFER_SIZE_OPTIONS = [
        __DEFAULT_SOCKET_BUFFER_SIZE,
        ['Custom size', False]
    ]

    __DEFAULT_AUDIO_DEVICE_TYPE = ['Input audio device', True]
    __AUDIO_DEVICE_OPTIONS = [
        __DEFAULT_AUDIO_DEVICE_TYPE,
        ['Output audio device', False]
    ]

    __DEFAULT_DEVICE_ENCODING_NAMES = [f'Locale of OS - {locale.getpreferredencoding()}', locale.getpreferredencoding()]
    __DEVICE_ENCODING_NAMES_OPTIONS = [
        __DEFAULT_DEVICE_ENCODING_NAMES,
        ['Use custom encoding', True]
    ]

    __DEFAULT_IGNORE_DEVICE_ENCODING_NAMES = ['Enable encoding', False]
    __IGNORE_DEVICE_ENCODING_NAMES_OPTIONS = [
        __DEFAULT_IGNORE_DEVICE_ENCODING_NAMES,
        ['Disable encoding', True]
    ]

    __DEFAULT_IS_ERROR_LOG = ['Disable error file log', False]
    __IS_ERROR_LOG_OPTIONS = [
        __DEFAULT_IS_ERROR_LOG,
        ['Enable error file log', True]]

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

    __DEFAULT_SAVE_PROFILE = ['Skip saving params', False]
    __SAVE_PROFILE_OPTIONS = [
        ['Save values to config file', True],
        __DEFAULT_SAVE_PROFILE
    ]

    @staticmethod
    def get_variants_config_load_options_data() -> OptionsData:
        return OptionsData(
            ["Load config", True],
            [
                ["Load config", True],
                ["Skip config", False]
            ]
        )

    @staticmethod
    def get_socket_buffer_size_options_data() -> OptionsData:
        return OptionsData(
            DefaultValuesAndOptions.__DEFAULT_SOCKET_BUFFER_SIZE,
            DefaultValuesAndOptions.__SOCKET_BUFFER_SIZE_OPTIONS
        )

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
    def get_encoding_charset_options_data() -> OptionsData:
        return OptionsData(
            DefaultValuesAndOptions.__DEFAULT_DEVICE_ENCODING_NAMES,
            DefaultValuesAndOptions.__DEVICE_ENCODING_NAMES_OPTIONS
        )

    @staticmethod
    def get_ignore_encoding_options_data() -> OptionsData:
        return OptionsData(
            DefaultValuesAndOptions.__DEFAULT_IGNORE_DEVICE_ENCODING_NAMES,
            DefaultValuesAndOptions.__IGNORE_DEVICE_ENCODING_NAMES_OPTIONS
        )

    @staticmethod
    def get_error_log_options_data() -> OptionsData:
        return OptionsData(
            DefaultValuesAndOptions.__DEFAULT_IS_ERROR_LOG,
            DefaultValuesAndOptions.__IS_ERROR_LOG_OPTIONS
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
    def get_save_profile_options_data() -> OptionsData:
        return OptionsData(
            DefaultValuesAndOptions.__DEFAULT_SAVE_PROFILE,
            DefaultValuesAndOptions.__SAVE_PROFILE_OPTIONS
        )

    @staticmethod
    def get_default_port() -> int:
        return DefaultValuesAndOptions.__DEFAULT_PORT

    @staticmethod
    def get_default_reconnect_attempt() -> int:
        return DefaultValuesAndOptions.__DEFAULT_RECONNECT_ATTEMPT

    @staticmethod
    def get_default_workers() -> int:
        return DefaultValuesAndOptions.__DEFAULT_WORKERS_COUNT

    @staticmethod
    def get_util_comparability_version() -> str:
        return version.__comparability_version__

    @staticmethod
    def get_heartbeat_delay() -> float:
        return DefaultValuesAndOptions.__DEFAULT_HEARTBEAT_DELAY

    @staticmethod
    def get_timeout() -> int:
        return DefaultValuesAndOptions.__DEFAULT_TIMEOUT
