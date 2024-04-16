from start_modes.options_data_creater import OptionsData


class DefaultValuesAndOptions:
    __DEFAULT_PORT = 6532
    __DEFAULT_WORKERS_COUNT = 2
    __DEFAULT_HEARTBEAT_ATTEMPT = 5

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

    __DEFAULT_THREAD_MODE = ['Multiprocessing mode (One CPU core per process may increase performance', False]
    __THREAD_MODE_OPTIONS = [
        ['Multithreading mode (GIL may reduce performance, because all threads works on one CPU core)', True],
        __DEFAULT_THREAD_MODE
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
    def get_thread_mode_options_data() -> OptionsData:
        return OptionsData(
            DefaultValuesAndOptions.__DEFAULT_THREAD_MODE,
            DefaultValuesAndOptions.__THREAD_MODE_OPTIONS
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
