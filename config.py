import configparser

def parse_config(config_path):
    config = configparser.ConfigParser()
    config.read(config_path)
    return config
