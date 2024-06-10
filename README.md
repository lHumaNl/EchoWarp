# EchoWarp

![EchoWarp_logo](EchoWarp.ico)

EchoWarp is a versatile network audio streaming tool designed to transmit audio streams between a server and a client
over a network. It supports both audio input and output devices, adapting to various audio streaming scenarios. EchoWarp
can operate in either server or client mode, with robust configuration options available via command-line arguments or
an interactive setup.

## Key Features

- **Cross-Platform Compatibility**: Works seamlessly on Windows, macOS, and Linux.
- **Flexible Audio Device Selection**: Choose specific input or output devices for audio streaming.
- **Real-Time Audio Streaming**: Utilizes UDP for transmitting audio data and TCP for control signals (on one port).
- **Robust Encryption and Integrity Checks**: Supports AES encryption and SHA-256 hashing to secure data streams.
- **Automatic Reconnection**: Implements heartbeat and authentication mechanisms to handle reconnections and ensure
  continuous streaming.
- **Configurable through CLI and Interactive Modes**: Offers easy setup through an interactive mode or scriptable CLI
  options.
- **Ban List Management**: Server maintains a ban list to block clients after a specified number of failed connection
  attempts.

## Important Notes

### Correct Usage

#### Running the Utility:

- The utility must be run both on the client and the server.
- **Start the Server First**: The server should be started first. If the client cannot connect to the server, it will
  throw an error.

#### Server and Client Roles:

- **Server**: The server is the host where the audio stream will be captured (e.g., the microphone on the host with
  Moonlight).
- **Client**: The client is the host where the captured audio stream from the server will be transmitted over the
  network (e.g., the host with Sunshine/Nvidia GameStream).

### Interactive Mode

- In interactive mode (where values are offered for selection), you do not need to manually enter default values. Simply
  leave the input field empty and press Enter, and the default value will be automatically accepted.

### Firewall and Ports

- Ensure that your firewall settings allow traffic through the necessary ports:
    - **Port**: Used for both audio streaming (UDP) and authentication/configuration exchange and connection status
      checking (TCP). Default is 4415.

### VPN Considerations

- If you are using a VPN, make sure it is configured to allow traffic through the necessary ports.
- Verify that the server IP address specified on the client is correct and accessible through the VPN.
- If you encounter connection issues, try disabling the VPN to see if the problem persists.

### Common Issues and Solutions

#### `[WinError 10049]`

- This error indicates that the specified address is not available on the interface. Possible solutions:
    - Check if the IP address specified is correct and reachable.
    - Ensure that the VPN is configured correctly and does not block the necessary port.
    - Verify that the firewall allows traffic on the specified port.

#### `timed out`

- This error indicates that the connection attempt to the specified address timed out. Possible solutions:
    - Ensure the server is running and accessible from the client.
    - Verify that the server IP address and port specified on the client are correct.
    - Check if the server's firewall allows incoming connections on the specified port.
    - Ensure that the VPN, if used, is properly configured to allow traffic through the necessary port.
    - Verify network stability and connectivity between the client and the server.
    - If the server has a dynamic IP address, ensure it hasn't changed since the last connection attempt.

#### `403 Forbidden (Client is banned)`

- This error indicates that the client has been banned by the server. Possible solutions:
    - Verify that the client is not making repeated failed connection attempts, which could lead to a ban.
    - Check the server's ban list to see if the client's IP address is listed and remove it if necessary.
    - Ensure that the client's configuration matches the server's requirements to avoid being banned.

#### `401 Unauthorized (Invalid password)`

- This error indicates that the password provided by the client is invalid. Possible solutions:
    - Ensure that the correct password is being used.
    - Verify that the password matches the one configured on the server.
    - Check for any typos or encoding issues in the password.

#### `409 Conflict (Client version mismatch)`

- This error indicates a version mismatch between the client and server. Possible solutions:
    - Ensure that both the client and server are running compatible versions of EchoWarp.
    - Update the client or server to the latest version to ensure compatibility.
    - Verify that the version specified in the client configuration matches the server's version.

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

| Argument                         | Description                                                                                                                                                   | Default                                                                                                                    |
|----------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------|
| `-c`, `--client`                 | Start utility in client mode (Mode in which the device receives an audio stream from the server over the network and plays it on the specified audio device). | Server mode (Mode in which the audio stream from the specified device is captured and sent to the client over the network) |
| `-o`, `--output`                 | Use the output audio device for streaming (like speakers and headphones).                                                                                     | Input device (like microphones)                                                                                            |
| `-u`, `--udp_port`               | Specify the UDP\TCP port for audio data transmission\authorization.                                                                                           | 4415                                                                                                                       |
| `-b`, `--socket_buffer_size`     | Size of socket buffer.                                                                                                                                        | 6144                                                                                                                       |
| `-d`, `--device_id`              | Specify the device ID directly to avoid interactive selection.                                                                                                | None                                                                                                                       |
| `-p`, `--password`               | Password for authentication.                                                                                                                                  | None                                                                                                                       |
| `-f`, `--config_file`            | Path to config file (ignoring other args, if they added).                                                                                                     | None                                                                                                                       |
| `-e`, `--device_encoding_names`  | Charset encoding for audio device.                                                                                                                            | System default encoding                                                                                                    |
| `--ignore_device_encoding_names` | Ignoring device names encoding.                                                                                                                               | False                                                                                                                      |
| `-l`, `--is_error_log`           | Write error log lines to file.                                                                                                                                | False                                                                                                                      |
| `-r`, `--reconnect`              | Number of failed connections (before closing the application in client mode\before ban client in server mode). (0=infinite)                                   | 5                                                                                                                          |
| `-s`, `--save_config`            | Save config file from selected args (ignoring default values).                                                                                                | None                                                                                                                       |
| `-a`, `--server_address`         | Specify the server address (only valid in client mode).                                                                                                       | None                                                                                                                       |
| `--ssl`                          | Enable SSL mode for encrypted communication (server mode only).                                                                                               | False                                                                                                                      |
| `-i`, `--integrity_control`      | Enable integrity control using hash (server mode only).                                                                                                       | False                                                                                                                      |
| `-w`, `--workers`                | Set the maximum number of worker threads (server mode only).                                                                                                  | 1                                                                                                                          |

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

1. **Failed Connection Attempts**: Each client connection attempt is monitored. If a client fails to authenticate
   multiple times (as specified by the `--reconnect` argument), the client is added to the ban list.
   Setting `--reconnect` to `0` in server mode disables the ban list feature.
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
python -m echowarp.main --client --server_address 192.168.1.5 --udp_port 6555
```