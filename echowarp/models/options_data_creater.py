from typing import List, Any


class OptionsFormatter:
    option_descr: str
    option_value: Any

    def __init__(self, option_list: List):
        self.option_descr = option_list[0]
        self.option_value = option_list[1]


class OptionsData:
    default_descr: str
    default_value: Any
    options: List[OptionsFormatter]

    def __init__(self, default_value_list: List, options: List[List]):
        self.default_descr = default_value_list[0]
        self.default_value = default_value_list[1]

        self.options = [OptionsFormatter(option) for option in options]
