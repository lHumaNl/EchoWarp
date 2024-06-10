import datetime
import locale
import logging


class Logger:
    IS_CORE_LOGGER_ENABLED: bool = False
    IS_WARNING_LOGGER_ENABLED: bool = False

    @staticmethod
    def init_core_logger():
        """
        Configures the logging settings for the application.
        Sets the logging level to INFO and specifies the format for log messages,
        including timestamp, log level, module, function name, and message.
        """
        if not Logger.IS_CORE_LOGGER_ENABLED:
            logging.basicConfig(
                level=logging.INFO,
                format='[%(asctime)s] %(levelname)s | '
                       'module: %(module)s | '
                       'funcName: %(funcName)s | '
                       '%(message)s',
                datefmt='%Y-%m-%d %H:%M:%S',
                encoding=locale.getpreferredencoding()
            )

            Logger.IS_CORE_LOGGER_ENABLED = True

    @staticmethod
    def init_warning_logger():
        """
        Configures an additional file handler for logging warnings.
        Log messages with level WARNING or higher will be written to the specified file.
        """
        if not Logger.IS_WARNING_LOGGER_ENABLED:
            current_date = datetime.datetime.now().strftime('%Y-%m-%d')
            file_handler = logging.FileHandler(
                f"echowarp_errors_{current_date}.log",
                encoding=locale.getpreferredencoding()
            )
            file_handler.setLevel(logging.WARNING)

            formatter = logging.Formatter('[%(asctime)s] %(levelname)s | '
                                          'module: %(module)s | '
                                          'funcName: %(funcName)s | '
                                          '%(message)s',
                                          datefmt='%Y-%m-%d %H:%M:%S')
            file_handler.setFormatter(formatter)

            logger = logging.getLogger()
            logger.addHandler(file_handler)

            Logger.IS_WARNING_LOGGER_ENABLED = True
