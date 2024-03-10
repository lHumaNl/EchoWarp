from logging_config import setup_logging

setup_logging()


def decode_string(string: str, encoding: str) -> str:
    try:
        device_name = string.encode(encoding).decode('utf-8')
    except UnicodeEncodeError:
        device_name = string

    return device_name
