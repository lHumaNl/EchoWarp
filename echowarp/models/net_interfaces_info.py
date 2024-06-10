import os
import socket
from dataclasses import dataclass
from typing import List, Tuple

import psutil


@dataclass
class InterfaceInfo:
    interface_name: str
    ip_address: str
    dns: str


class NetInterfacesInfo:
    __port: int
    __interface_info_list: List[InterfaceInfo]
    __is_different_dns: bool

    def __init__(self, port: int):
        self.__port = port
        self.__interface_info_list, self.__is_different_dns = self.__get_net_interfaces_list()

    def get_formatted_info_str(self) -> str:
        if len(self.__interface_info_list) == 1:
            formatted_str = [
                    (f'Interface name: {interface.interface_name}, '
                     f'IP address: {interface.ip_address}:{self.__port}, '
                     f'DNS: {interface.dns}:{self.__port}')
                    for interface in self.__interface_info_list
                ]
        else:
            if self.__is_different_dns:
                formatted_str = [
                    (f'Interface name: {interface.interface_name}, '
                     f'IP address: {interface.ip_address}:{self.__port}, '
                     f'DNS: {interface.dns}:{self.__port}')
                    for interface in self.__interface_info_list
                ]
            else:
                formatted_str = [
                    (f'Interface name: {interface.interface_name}, '
                     f'IP address: {interface.ip_address}:{self.__port}')
                    for interface in self.__interface_info_list
                ]
                formatted_str.append(f'DNS: {socket.getfqdn()}:{self.__port}')

        return os.linesep.join(formatted_str)

    @staticmethod
    def __get_net_interfaces_list() -> Tuple[List[InterfaceInfo], bool]:
        addresses = psutil.net_if_addrs()
        interface_info_list: List[InterfaceInfo] = []
        default_dns = socket.getfqdn()

        for interface_name, interface_addresses in addresses.items():
            for address in interface_addresses:
                if address.family == socket.AddressFamily.AF_INET:
                    ip_address = address.address
                    if (ip_address == '127.0.0.1' or
                            (ip_address.startswith('169.254.') and address.netmask == '255.255.0.0')):
                        continue

                    try:
                        dns_name = socket.gethostbyaddr(ip_address)[0]
                    except socket.herror:
                        dns_name = None

                    if dns_name is None or dns_name.strip() == '':
                        dns_name = default_dns

                    interface_info_list.append(InterfaceInfo(interface_name, ip_address, dns_name))

        return interface_info_list, any(interface.dns != default_dns for interface in interface_info_list)
