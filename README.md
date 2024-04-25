# EchoWarp

EchoWarp is a versatile network audio streaming tool designed to transmit audio streams between a server and a client
over a network. It supports both audio input and output devices, adapting to various audio streaming scenarios. EchoWarp
can operate in either server or client mode, with robust configuration options available via command-line arguments or
an interactive setup.

## Key Features

- **Cross-Platform Compatibility**: Works seamlessly on Windows, macOS, and Linux.
- **Flexible Audio Device Selection**: Choose specific input or output devices for audio streaming.
- **Real-Time Audio Streaming**: Utilizes UDP for transmitting audio data and TCP for control signals.
- **Robust Encryption and Integrity Checks**: Supports AES encryption and SHA-256 hashing to secure data streams.
- **Automatic Reconnection**: Implements heartbeat and authentication mechanisms to handle reconnections and ensure
  continuous streaming.
- **Configurable through CLI and Interactive Modes**: Offers easy setup through an interactive mode or scriptable CLI
  options.

## Prerequisites

- **Operating System**: Windows, Linux, or macOS.
- **Python Version**: Python 3.6 or later.
- **Dependencies**: Includes libraries like PyAudio, and Cryptography.

## PyPi Repository

EchoWarp is also available as a package on the Python Package Index (PyPi), which simplifies the installation process
and manages dependencies automatically. This is the recommended method if you wish to include EchoWarp in your Python
project.

### Features of Installing via PyPi:

- **Automatic Dependency Management**: All required libraries, such as PyAudio and Cryptography, are automatically
  installed.
- **Easy Updates**: Simplifies the process of obtaining the latest version of EchoWarp with a simple pip command.
- **Isolation from System Python**: Installing via a virtual environment prevents any conflicts with system-wide Python
  packages.

### Installation with PyPi (pip)

To install EchoWarp using pip, follow these steps:

1. Set up a Python virtual environment (optional but recommended):

```bash
   python -m venv .venv
   source .venv/bin/activate  # On Windows, use `.\.venv\Scripts\activate`
```

2. Install EchoWarp using pip:

```bash
   pip install echowarp
```

### Updating EchoWarp

To update to the latest version of EchoWarp, simply run:

```bash
   pip install --upgrade echowarp
```

This ensures that you have the latest features and improvements.

For more information and assistance, you can visit the EchoWarp PyPi page:
https://pypi.org/project/echowarp/

## Installation from source

Clone the repository and set up a Python virtual environment:

```bash
git clone https://github.com/lHumaNl/EchoWarp.git
cd EchoWarp
python -m venv venv
source venv/bin/activate  # On Windows, use `.\venv\Scripts\activate`
pip install -r requirements.txt
```

## Usage

EchoWarp can be launched in either server or client mode, with settings configured interactively or via command-line
arguments.

## Interactive Mode

Simply run the main.py script without arguments to enter interactive mode:

```bash
python main.py
```

Follow the on-screen prompts to configure the utility.

## Command-Line Arguments

EchoWarp supports configuration via command-line arguments for easy integration into scripts and automation workflows.

#### Arguments Table

| Argument                    | Description                                                                       | Default      |
|-----------------------------|-----------------------------------------------------------------------------------|--------------|
| `-c`, `--client`            | Start utility in client mode.                                                     | Server mode  |
| `-o`, `--output`            | Use the output audio device for streaming.                                        | Input device |
| `-p`, `--udp_port`          | Specify the UDP port for audio data transmission.                                 | 4415         |
| `-d`, `--device_id`         | Specify the device ID directly to avoid interactive selection.                    | None         |
| `-a`, `--server_addr`       | Specify the server address (only valid in client mode).                           | None         |
| `-b`, `--heartbeat`         | Set the number of allowed missed heartbeats before disconnect (server mode only). | 5            |
| `--ssl`                     | Enable SSL mode for encrypted communication (server mode only).                   | False        |
| `-i`, `--integrity_control` | Enable integrity control using hash (server mode only).                           | False        |
| `-w`, `--workers`           | Set the maximum number of worker threads (server mode only).                      | 1            |

Use these arguments to configure the utility directly from the command line for both automation and manual setups.

## Examples

### Consult the help output for detailed command-line options::

```bash
python main.py --help
```

### Client Mode with Custom Server Address and Port:

```bash
python main.py --client --server_addr 192.168.1.5 --udp_port 6555
```