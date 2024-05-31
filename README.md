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
- **Ban List Management**: Server maintains a ban list to block clients after a specified number of failed connection
  attempts.

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
python -m echowarp.main
```

Follow the on-screen prompts to configure the utility.

## Command-Line Arguments

EchoWarp supports configuration via command-line arguments for easy integration into scripts and automation workflows.

#### Arguments Table

| Argument                         | Description                                                     | Default                 |
|----------------------------------|-----------------------------------------------------------------|-------------------------|
| `-c`, `--client`                 | Start utility in client mode.                                   | Server mode             |
| `-o`, `--output`                 | Use the output audio device for streaming.                      | Input device            |
| `-u`, `--udp_port`               | Specify the UDP port for audio data transmission.               | 4415                    |
| `-b`, `--socket_buffer_size`     | Size of socket buffer.                                          | 6144                    |
| `-d`, `--device_id`              | Specify the device ID directly to avoid interactive selection.  | None                    |
| `-p`, `--password`               | Password for authentication.                                    | None                    |
| `-f`, `--config_file`            | Path to config file (ignoring other args, if they added).       | None                    |
| `-e`, `--device_encoding_names`  | Charset encoding for audio device.                              | System default encoding |
| `--ignore_device_encoding_names` | Ignoring device names encoding.                                 | False                   |
| `-l`, `--is_error_log`           | Init error file logger.                                         | False                   |
| `-r`, `--reconnect`              | Number of failed connections before closing the application.    | 5                       |
| `-s`, `--save_config`            | Save config file from selected args (ignoring default values).  | None                    |
| `-a`, `--server_address`         | Specify the server address (only valid in client mode).         | None                    |
| `--ssl`                          | Enable SSL mode for encrypted communication (server mode only). | False                   |
| `-i`, `--integrity_control`      | Enable integrity control using hash (server mode only).         | False                   |
| `-w`, `--workers`                | Set the maximum number of worker threads (server mode only).    | 1                       |

Use these arguments to configure the utility directly from the command line for both automation and manual setups.

## Launching with Config File

You can launch EchoWarp using a configuration file to avoid specifying all the arguments manually. Create a
configuration file with the desired settings and use the --config_file argument to specify the path to this file.

Example of a configuration file (config.conf):

```ini
[echowarp_conf]
is_server=False
udp_port=6532
socket_buffer_size=10240
device_id=1
password=mysecretpassword
device_encoding_names=cp1251
is_error_log=True
server_address=192.168.1.5
reconnect_attempt=15
is_ssl=True
is_integrity_control=True
```

To launch EchoWarp with the configuration file with CLI args:

```bash
python -m echowarp.main --config_file path/to/config.conf
```

## Ban List Management

EchoWarp server maintains a ban list to block clients after a specified number of failed connection attempts. This
feature helps to secure the server from unauthorized access attempts and repeated failed authentications.

### How Ban List Works

1. **Failed Connection Attempts**: Each client connection attempt is monitored. If a client fails to authenticate multiple times (as specified by the `--reconnect` argument), the client is added to the ban list. Setting `--reconnect` to `0` in server mode disables the ban list feature.
2. **Banning Clients**: Once a client is banned, it cannot reconnect to the server until the ban list is manually
   cleared or the server is restarted.
3. **Persistent Ban List**: The ban list is saved to a file (`echowarp_ban_list.txt`) to maintain the list of banned
   clients across server restarts.

### Example of Ban List File

The ban list file (`echowarp_ban_list.txt`) contains the IP addresses of banned clients, one per line:

```txt
192.168.1.100
192.168.1.101
```

## Additional Features

- **Heartbeat Mechanism**: Ensures the continuous connection between client and server by periodically sending heartbeat
  messages.
- **Thread Pooling**: Utilizes thread pooling for handling concurrent tasks efficiently.
- **Logging**: Comprehensive logging for both normal operations and errors, with an option to enable error logging to a
  file.

## Examples

### Consult the help output for detailed command-line options::

```bash
python -m echowarp.main --help
```

### Client Mode with Custom Server Address and Port:

```bash
python -m echowarp.main --client --server_addr 192.168.1.5 --udp_port 6555
```