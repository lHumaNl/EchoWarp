import logging
import os
from typing import Dict

from echowarp.models.default_values_and_options import DefaultValuesAndOptions
from echowarp.start_modes.config_parser import ConfigParser


class ClientStatus:
    __reconnect_attempts: int
    __is_banned: bool
    __is_first_time_message: bool
    __failed_connect_attempts: int
    __all_failed_connect_attempts: int
    __success_connect_attempts: int

    def __init__(self, is_banned: bool, reconnect_attempts: int):
        self.__reconnect_attempts = reconnect_attempts
        self.__is_banned = is_banned
        self.__is_first_time_message = True

        if is_banned:
            self.__failed_connect_attempts = reconnect_attempts
            self.__all_failed_connect_attempts = reconnect_attempts
        else:
            self.__failed_connect_attempts = 0
            self.__all_failed_connect_attempts = 0

        self.__success_connect_attempts = 0

    def is_banned(self) -> bool:
        return self.__is_banned

    def success_connect_attempt(self):
        self.__failed_connect_attempts = 0
        self.__success_connect_attempts += 1
        self.__is_banned = False
        self.__is_first_time_message = True

    def get_failed_connect_attempts(self) -> int:
        return self.__failed_connect_attempts

    def get_success_connect_attempts(self) -> int:
        return self.__success_connect_attempts

    def get_all_failed_connect_attempts(self) -> int:
        return self.__all_failed_connect_attempts

    def fail_connect_attempt(self):
        if not self.__is_banned:
            self.__failed_connect_attempts += 1
            self.__all_failed_connect_attempts += 1

        if self.__failed_connect_attempts >= self.__reconnect_attempts > 0:
            self.__is_banned = True

    def is_first_time_message(self) -> bool:
        if self.__is_first_time_message:
            self.__is_first_time_message = False

            return True
        else:
            return self.__is_first_time_message


class BanList:
    __ban_list: Dict[str, ClientStatus]
    __reconnect_attempts: int

    def __init__(self, reconnect_attempts: int):
        self.__reconnect_attempts = reconnect_attempts
        self.__get_ban_list()

    def is_banned(self, client_ip: str) -> bool:
        if client_ip in self.__ban_list:
            return self.__ban_list[client_ip].is_banned()
        else:
            return False

    def success_connect_attempt(self, client_ip: str):
        if client_ip in self.__ban_list:
            self.__ban_list[client_ip].success_connect_attempt()

    def fail_connect_attempt(self, client_ip: str):
        if client_ip in self.__ban_list:
            self.__ban_list[client_ip].fail_connect_attempt()

    def add_client_to_ban_list(self, client_ip: str):
        if client_ip not in self.__ban_list:
            self.__ban_list[client_ip] = ClientStatus(False, self.__reconnect_attempts)

    def is_first_time_message(self, client_ip: str) -> bool:
        if client_ip in self.__ban_list:
            return self.__ban_list[client_ip].is_first_time_message()
        else:
            return True

    def get_failed_connect_attempts(self, client_ip: str) -> int:
        return self.__ban_list[client_ip].get_failed_connect_attempts()

    def get_success_connect_attempts(self, client_ip: str) -> int:
        return self.__ban_list[client_ip].get_success_connect_attempts()

    def get_all_failed_connect_attempts(self, client_ip: str) -> int:
        return self.__ban_list[client_ip].get_all_failed_connect_attempts()

    def update_ban_list_file(self):
        if self.__reconnect_attempts <= 0:
            return

        banned_clients = [ip for ip, client in self.__ban_list.items()
                          if client.is_banned()]

        if len(banned_clients) > 0:
            try:
                with open(DefaultValuesAndOptions.BAN_LIST_FILE, 'w', encoding='utf-8') as file:
                    file.write(os.linesep.join(banned_clients))
            except Exception as e:
                logging.error(f'Failed to update ban list file: {e}')

    def __get_ban_list(self):
        self.__ban_list = {}

        if os.path.isfile(DefaultValuesAndOptions.BAN_LIST_FILE) and self.__reconnect_attempts > 0:
            str_lines = ConfigParser.get_file_str(DefaultValuesAndOptions.BAN_LIST_FILE).split(os.linesep)
            for line in str_lines:
                line = line.strip()
                if line != '' and line is not None:
                    self.__ban_list[line] = ClientStatus(True, self.__reconnect_attempts)
