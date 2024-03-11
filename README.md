# EchoWarp

EchoWarp is a network audio streaming utility designed to stream audio from a server to a client over a network. It
supports both input and output audio devices, providing flexibility for various audio streaming scenarios. EchoWarp
operates in either server or client mode, with a background thread handling authentication and heartbeat checks to
ensure a stable connection.

## Features

- **Audio Streaming**: Stream audio in real-time over the network.
- **Server & Client Modes**: Operate as either a server (sending audio) or a client (receiving audio).
- **Device Selection**: Choose specific input or output audio devices for streaming.
- **Authentication and Heartbeat**: Secure connection establishment with heartbeat monitoring for connection integrity.
- **Interactive and Argument Modes**: Configure settings interactively or via command-line arguments for flexibility in
  different use cases.

## Installation

Before running EchoWarp, ensure you have Python 3.6+ installed on your system. Clone this repository and install the
required dependencies:

```bash
git clone https://github.com/lHumaNl/EchoWarp.git
cd EchoWarp
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
| Argument              | Description                                     | Required | Default Value |
|-----------------------|-------------------------------------------------|----------|---------------|
| `-s`, `--server`      | Launch EchoWarp in server mode.                 | No       | Client mode   |
| `-o`, `--output`      | Use the output audio device.                    | No       | Input device  |
| `-a`, `--server_addr` | Specify the server address (Client mode only).  | Yes*     | -             |
| `-p`, `--udp_port`    | UDP port for audio streaming.                   | No       | 6532          |
| `-e`, `--encoding`    | Encoding for device names.                      | No       | cp1251        |
| `-b`, `--heartbeat`   | Number of heartbeat attempts before disconnect. | No       | 5             |

\* Required only in client mode.

## Examples

### Server Mode with Default Settings:

```bash
python main.py --server
```

### Client Mode with Custom Server Address and Port:

```bash
python main.py --server_addr 192.168.1.5 --udp_port 6555
```

## Dependencies

- **PyAudio**: For audio capture and playback.
- **Opuslib**: For audio compression and decompression.