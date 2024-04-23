import logging


def setup_logging():
    """
        Configures the logging settings for the application.
        Sets the logging level to INFO and specifies the format for log messages,
        including timestamp, log level, module, function name, and message.
    """
    logging.basicConfig(level=logging.INFO,
                        format='[%(asctime)s] %(levelname)s | '
                               'module: %(module)s | '
                               'funcName: %(funcName)s | '
                               'message: %(message)s',
                        datefmt='%Y-%m-%d %H:%M:%S')
