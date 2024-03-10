import logging


def setup_logging():
    logging.basicConfig(level=logging.INFO,
                        format='[%(asctime)s] %(levelname)s | '
                               'module: %(module)s | '
                               'funcName: %(funcName)s | '
                               'message: %(message)s',
                        datefmt='%Y-%m-%d %H:%M:%S')
