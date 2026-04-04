<p align="center">
  <img src="build/icons/icon_512.png" alt="EchoWarp Logo" width="128">
</p>

<h1 align="center">EchoWarp</h1>

<p align="center">
  Real-time network audio streaming between hosts
</p>

<p align="center">
  <a href="README.md">English</a> |
  <a href="docs/translations/README.ru.md">Русский</a> |
  <a href="docs/translations/README.zh.md">简体中文</a> |
  <a href="docs/translations/README.es.md">Español</a> |
  <a href="docs/translations/README.fr.md">Français</a> |
  <a href="docs/translations/README.it.md">Italiano</a> |
  <a href="docs/translations/README.ko.md">한국어</a> |
  <a href="docs/translations/README.ja.md">日本語</a> |
  <a href="docs/translations/README.de.md">Deutsch</a> |
  <a href="docs/translations/README.pt.md">Português</a> |
  <a href="docs/translations/README.tr.md">Türkçe</a> |
  <a href="docs/translations/README.vi.md">Tiếng Việт</a> |
  <a href="docs/translations/README.pl.md">Polski</a>
</p>

<p align="center">
  <a href="https://github.com/lHumaNl/EchoWarp/releases"><img src="https://img.shields.io/github/v/release/lHumaNl/EchoWarp?style=flat-square" alt="Release"></a>
  <a href="https://github.com/lHumaNl/EchoWarp/actions"><img src="https://img.shields.io/github/actions/workflow/status/lHumaNl/EchoWarp/ci.yml?branch=main&style=flat-square" alt="CI"></a>
  <a href="https://github.com/lHumaNl/EchoWarp/blob/main/LICENSE"><img src="https://img.shields.io/github/license/lHumaNl/EchoWarp?style=flat-square" alt="License"></a>
  <a href="https://github.com/lHumaNl/EchoWarp/releases"><img src="https://img.shields.io/github/downloads/lHumaNl/EchoWarp/total?style=flat-square" alt="Downloads"></a>
</p>

---

Capture audio on one machine, play it on another — in real time over the network. EchoWarp uses WebRTC for transport and Opus for compression, delivering low-latency audio with end-to-end encryption.

## Features

- **1:1 streaming** — server captures, client plays (or reverse)
- **Duplex mode** — bidirectional audio, both sides hear each other
- **Broadcast (1:N)** — one server streams to multiple clients
- **Conference (N:N)** — multi-participant mixing with personal mixes (total minus self)
- **Virtual microphone** — received audio appears as a mic input (Discord, Zoom, OBS)
- **LAN discovery** — automatic server detection via mDNS
- **Recording** — WAV recording: mixed, per-track, or both
- **SIMD acceleration** — AVX/SSE (x86) and NEON (ARM) for audio mixing
- **End-to-end encryption** — AES + DTLS/SRTP
- **Cross-platform** — macOS, Linux, Windows

## Quick Start

1. Download a pre-built binary from [Releases](https://github.com/lHumaNl/EchoWarp/releases) (no dependencies — opus is statically linked)
2. Run `EchoWarp server` or `EchoWarp client`
3. Configure everything in the interactive TUI and press Enter to start

> **Windows users:** Download [launcher scripts](https://github.com/lHumaNl/EchoWarp/tree/main/build/launchers) and place them next to `EchoWarp.exe`. Double-click `EchoWarp.bat` to launch the TUI menu — no need to open cmd.exe manually.

## Interactive TUI

The TUI is the primary interface for EchoWarp. Just run `EchoWarp server` or `EchoWarp client` — the interactive setup screen opens automatically.

### Setup Screen

The setup screen has a two-column layout:

**Left column — Audio devices.** Lists all available input/output devices with their properties (channels, sample rate, bit depth). In duplex or conference mode you assign separate capture `[C]` and playback `[P]` roles. Virtual device creation is available directly from the list.

**Right column — Settings.** All session parameters are configurable here with inline validation:

| Setting | Description |
|---------|-------------|
| Server address | Server IP/hostname (client only) |
| Port | TCP port (default: 4415) |
| Password | Authentication password |
| Mode | Normal / Reverse / Duplex |
| Max clients | Maximum connections (server only) |
| Sample rate | 8000 / 16000 / 24000 / 48000 Hz |
| Channels | Mono / Stereo |
| Opus bitrate | 16–512 kbps |
| Echo cancellation | AEC for duplex mode |
| TLS | Enable/disable with cert and key paths |
| Log level | debug / info / warn / error |

Navigate between columns with `Tab`, move through fields with arrow keys, and press `Enter` to confirm and start streaming.

### LAN Discovery

On the client setup screen, press `Ctrl+F` to open the discovery overlay. EchoWarp uses mDNS to automatically find servers on the local network — select one from the list and the address/port fields are filled in automatically.

### Device Profiles

Press `Ctrl+O` to open the profiles overlay. Save your current device configuration as a named profile, load a previously saved one, or delete profiles you no longer need. Profiles remember device assignments and roles.

### Streaming Screen

Once connected, the TUI shows real-time streaming status:

- **Connection stats** — RTT, jitter, packet loss with threshold bars showing proximity to quality limits
- **Connection quality** — overall indicator (Excellent / Good / Fair / Poor) calculated from RTT, jitter, and loss
- **Spectrum analyzer** — real-time FFT frequency visualization of the audio signal
- **VU meters** — per-channel audio level meters
- **Bitrate** — live upload/download bitrate display
- **Device controls** — volume adjustment, mute, device switching
- **Log panel** — toggleable scrollable log view

### TUI Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `Tab` | Switch columns (setup) |
| `Ctrl+F` | LAN discovery (client setup) |
| `Ctrl+O` | Device profiles (setup) |
| `Enter` | Confirm and start |
| `Ctrl+Q` | Quit |
| `Ctrl+P` | Pause/resume streaming |
| `Ctrl+L` | Toggle log panel |
| `Ctrl+M` | Mute/unmute |
| `+` / `-` | Volume up/down |
| `Ctrl+Up/Down` | Scroll logs |
| `Ctrl+R` | Start/stop recording |
| `Ctrl+D` | Kick client (server) |
| `Ctrl+B` | Ban client (server) |

## Streaming Modes

EchoWarp has four streaming modes. Choose the one that fits your scenario — the mode is selected in the TUI setup screen (server side) or detected automatically (client side).

### Normal — One-Way: Server to Clients

The server captures audio and sends it to all connected clients. Clients only listen.

```
Server [captures audio] ───→ Client 1 [plays audio]
                         ├──→ Client 2 [plays audio]
                         └──→ Client N [plays audio]
```

**When to use:** stream music/podcasts to listeners, broadcast system audio (YouTube, Spotify) to another room, send a loopback feed to a remote machine.

**Devices needed:**

| Side | Capture (input) | Playback (output) |
|------|:-:|:-:|
| Server | required | — |
| Client | — | required |

**Participants:** 1 server + 1…N clients (set *Max clients* in the TUI).

---

### Reverse — One-Way: Clients to Server

The opposite of Normal. Clients capture audio and send it to the server. The server only listens.

```
Client 1 [captures audio] ───→ Server [plays audio]
Client 2 [captures audio] ──┘
```

**When to use:** use a remote microphone on the server machine, collect audio from a remote location, send a client's mic to the server speakers.

**Devices needed:**

| Side | Capture (input) | Playback (output) |
|------|:-:|:-:|
| Server | — | required |
| Client | required | — |

**Participants:** 1 server + 1…N clients.

---

### Duplex — Two-Way

Both sides capture and play simultaneously. Everyone hears everyone.

```
Server [mic + speakers] ⟷ Client 1 [mic + speakers]
                        ⟷ Client 2 [mic + speakers]
```

**When to use:** voice call between two or more machines, intercom between rooms, collaborative audio session.

**Devices needed:**

| Side | Capture (input) | Playback (output) |
|------|:-:|:-:|
| Server | required | required |
| Client | required | required |

**Participants:** 1 server + 1…N clients. Supports AEC (echo cancellation) to prevent feedback loops.

---

### Conference — Multi-User Mixing (N:N)

The server acts as a mixing hub. Each participant (server + clients) sends their audio and receives a personal mix of **everyone else** (total minus self). This prevents hearing your own voice.

```
Client 1 ──┐              ┌──→ Client 1 (hears Server + 2 + 3)
Client 2 ──┤  Server hub  ├──→ Client 2 (hears Server + 1 + 3)
Client 3 ──┘              └──→ Client 3 (hears Server + 1 + 2)
```

**When to use:** group calls with 3+ machines, remote rehearsals, multi-participant conferences.

**Devices needed:**

| Side | Capture (input) | Playback (output) |
|------|:-:|:-:|
| Server (hub-only) | — | — |
| Server (participant) | required | required |
| Client | required | required |

The server can work as a **hub-only** (no local audio — just mixing for clients) or as a full participant with its own mic and speakers.

**Participants:** 1 server + 2…N clients. Supports recording (mixed, per-track, or both), kick/ban, and per-participant volume control.

---

### Mode Summary

| | Normal | Reverse | Duplex | Conference |
|---|---|---|---|---|
| **Direction** | Server → Clients | Clients → Server | Both ways | Everyone ⟷ Everyone |
| **Server devices** | Capture only | Playback only | Both | Both (or none if hub) |
| **Client devices** | Playback only | Capture only | Both | Both |
| **Max participants** | 1 + N | 1 + N | 1 + N | 1 + N |
| **AEC support** | — | — | yes | yes |
| **Personal mix** | — | — | — | yes |
| **Recording** | — | — | — | yes |
| **CLI flag** | *(default)* | `-r` | `-X` | `--conference` |

> **Tip:** The number of clients is controlled by the *Max clients* setting in the TUI (default: 1). Set it higher if you need broadcast or multi-user scenarios.

## Audio Routing Guide

### Use Cases

EchoWarp supports several audio routing scenarios. The table below shows which mode and device setup to use for each:

| Scenario | Mode | Server devices | Client devices |
|----------|------|----------------|----------------|
| Stream mic to remote speakers | Normal | Mic `[Input]` | Speakers `[Output]` |
| Stream system audio (YouTube, Spotify) | Normal | 🔄 Loopback `[Input]` | Speakers `[Output]` |
| Use remote mic in Discord/Zoom | Reverse | Speakers `[Output]` | Mic `[Input]` |
| Bidirectional voice chat | Duplex | Mic + Speakers | Mic + Speakers |
| Group call (3+ machines) | Conference | — (hub) | Mic + Speakers |
| Remote audio as virtual mic in OBS | Normal | 🔄 Loopback `[Input]` | ⟡ Virtual mic `[Output]` |
| Stream + local mic as one virtual mic | Normal | Mic `[Input]` | ⟡ Virtual `[Output]` + 🎤 Mix mic |

### Loopback — Capture System Audio

Loopback devices capture all audio playing on a machine (music, video calls, game sound) and make it available as an input source.

| Platform | How it works | Setup |
|----------|-------------|-------|
| **macOS** | Aggregate Device (speakers + BlackHole) | Install [BlackHole](https://github.com/ExistentialAudio/BlackHole): `brew install blackhole-2ch`. Loopback devices appear in the TUI automatically |
| **Windows** | WASAPI native loopback | Built-in, no extra software needed |
| **Linux** | PulseAudio monitor sources | Built-in with PulseAudio/PipeWire |

In the TUI, loopback devices are marked with 🔄 and appear in the Input section.

### Virtual Microphone — Route Audio to Other Apps

A virtual microphone makes received audio appear as a mic input that Discord, Zoom, OBS, and other applications can use.

| Platform | Driver | Install |
|----------|--------|---------|
| **macOS** | [BlackHole](https://github.com/ExistentialAudio/BlackHole) | `brew install blackhole-2ch` |
| **Windows** | [VB-Audio Virtual Cable](https://vb-audio.com/Cable/) | Download from vb-audio.com |
| **Linux** | PulseAudio null-sink | Created from TUI: Settings → Virtual mic → Create |

On macOS and Windows, install the driver and select it in the Output section of the TUI. On Linux, the TUI creates the virtual sink automatically — in other apps, select **"Monitor of EchoWarp"** as the microphone.

Virtual devices are marked with ⟡ in the TUI and show **adaptive** instead of a fixed sample rate.

### Mixing Local Microphone into Virtual Output

When you route a remote stream to a virtual output device (BlackHole, VB-Cable), third-party apps like Discord or Zoom see only the stream — they don't hear your local microphone. If you need both your voice **and** the stream to appear as a single mic input, EchoWarp can mix them together.

**How to use:** In the TUI, select a virtual device in the Output section. Input devices will appear below it as sub-items — check the microphone you want to mix in:

```
🔊 Output
  ⟡ BlackHole 2ch                           stereo 32-bit adaptive   [✓]
      └─ 🎤 Built-in Microphone                                      [✓]
```

The mixed audio (stream + mic) is written to the virtual output. In Discord/Zoom/OBS, select that virtual device as your microphone — both the stream and your voice will be heard.

**OS-level alternatives** (without using the built-in mixer):

| Platform | How | Details |
|----------|-----|---------|
| **macOS** | Aggregate Device | Open Audio MIDI Setup → Create Aggregate Device combining your mic + BlackHole. Select the aggregate as input in your app |
| **Windows** | [VoiceMeeter](https://vb-audio.com/Voicemeeter/) | Free virtual mixer — route both VB-Cable and your mic into VoiceMeeter, use its output as the mic in your app |
| **Linux** | PulseAudio combine-source | `pactl load-module module-combine-source source_name=combined slaves=<mic>,<virtual>.monitor` |

The built-in mixer works on all platforms and requires no additional configuration.

### Device Sections

The TUI shows only the sections relevant to the current mode:

| Mode + Side | Visible sections |
|-------------|-----------------|
| Normal server / Reverse client | 🎤 Input only |
| Normal client / Reverse server | 🔊 Output only |
| Duplex / Conference | 🎤 Input + 🔊 Output |

### Diagnostics

Run `EchoWarp doctor` to check your audio setup:

```
Audio
[OK] Audio system: 2 input device(s), 2 output device(s)
[OK] Virtual audio: BlackHole 2ch detected
```

If no virtual audio driver is found, the doctor will show installation instructions for your platform.

### FAQ

**I want to listen to music from my computer on speakers in another room.**
→ Run `EchoWarp server` on the music machine, select the 🔄 loopback device (your speakers) in Input. Run `EchoWarp client` on the remote machine, select speakers in Output.

**I want to use a remote microphone for Zoom/Discord.**
→ Run `EchoWarp server` in Reverse mode on the remote machine (the one with the mic). Run `EchoWarp client` on your machine, select a virtual audio device (BlackHole/VB-Cable) in Output. In Zoom/Discord, select that virtual device as your microphone.

**I want to stream system audio (YouTube, Spotify) to another machine.**
→ Same as "music in another room" — use 🔄 loopback on the server side. The loopback device captures everything playing through the selected speakers.

**I want remote audio to appear as a mic input in OBS.**
→ Run the server with the audio source. On your machine (client), select a virtual audio device in Output. In OBS, add an "Audio Input Capture" source and select the virtual device.

**I want a two-way voice chat between two computers.**
→ Use Duplex mode. On both machines, select a microphone in Input and speakers in Output.

**I want a group call with 3+ machines.**
→ Use Conference mode. The server acts as a hub (no devices needed). Each client selects a mic and speakers. Everyone hears everyone else, minus their own voice.

**I want Discord to hear both the remote stream AND my voice through one virtual mic.**
→ On the client, select a virtual device (BlackHole/VB-Cable) in Output. A list of input devices will appear below it — check your microphone. EchoWarp will mix the stream and your mic into the virtual output. In Discord, select the virtual device as your microphone.

**I don't have BlackHole / VB-Cable installed.**
→ Run `EchoWarp doctor` — it will tell you exactly what to install and how. On Linux, no extra software is needed.

**The loopback device doesn't appear in the TUI.**
→ macOS: Install BlackHole first (`brew install blackhole-2ch`). Windows/Linux: loopback devices are built-in and should appear automatically.

**The virtual device shows "adaptive" instead of a sample rate.**
→ This is normal. Virtual audio drivers (BlackHole, VB-Cable) adapt to whatever sample rate the application uses — the displayed rate is not meaningful.

## CLI Mode

All settings available in the TUI can also be passed as CLI flags for scripting and automation:

```bash
# Server
EchoWarp server -d 1 -P mypassword

# Client (auto-discovers server on LAN if -a is omitted)
EchoWarp client -a 192.168.1.10 -d 2 -P mypassword

# List audio devices
EchoWarp devices
```

### Modes

See [Streaming Modes](#streaming-modes) for detailed descriptions of each mode.

| Mode | Flag |
|------|------|
| Normal | *(default)* |
| Reverse | `-r` |
| Duplex | `-X` |
| Conference | `--conference` |

### Common Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--device` | `-d` | Audio device ID |
| `--device-name` | `-D` | Select device by name (substring match) |
| `--password` | `-P` | Authentication password |
| `--port` | `-p` | TCP port (default: 4415) |
| `--sample-rate` | | Sample rate (default: 48000) |
| `--channels` | | 1=mono, 2=stereo (default: 1) |
| `--max-clients` | | Max connected clients (default: 1) |
| `--virtual-mic` | | Create virtual microphone device |
| `--config` | `-c` | Load YAML config file |
| `--save-config` | `-s` | Save current settings to YAML |
| `--profile` | | Load saved device profile |
| `--stun-server` | | Custom STUN servers for NAT traversal |
| `--tls-cert` / `--tls-key` | | TLS certificate and key |
| `--log-level` | | debug, info, warn, error |
| `--aec` | | Acoustic echo cancellation (duplex) |
| `--loopback` | | Capture system audio (macOS, requires BlackHole) |
| `--dry-run` | | Validate config and exit |

### Configuration

Settings priority: CLI flags > environment variables (`ECHOWARP_*`) > config file > defaults.

The TUI setup screen allows you to save and load configuration files interactively. You can also use CLI flags:

```bash
# Save current settings to a file
EchoWarp server -d 1 -P pass --channels 2 -s myconfig.yml

# Load settings from a file
EchoWarp server -c myconfig.yml
```

## Installation

### Pre-built binaries

Download from the [Releases](https://github.com/lHumaNl/EchoWarp/releases) page. Available for:
- **macOS** — arm64 (Apple Silicon), amd64 (Intel), includes `.app` bundle
- **Linux** — amd64, arm64, armv7, armv6
- **Windows** — amd64, arm64

### Build from source

Requires Go 1.22+ and libopus development headers.

```bash
# macOS
brew install opus pkg-config

# Ubuntu/Debian
sudo apt-get install libopus-dev pkg-config

# Fedora
sudo dnf install opus-devel pkgconfig
```

```bash
git clone https://github.com/lHumaNl/EchoWarp.git
cd EchoWarp
make build
```

The resulting binary statically links opus — no runtime dependency needed.

## System Requirements

- **macOS** 11.0+ (arm64, amd64)
- **Linux** (amd64, arm64, armv7, armv6)
- **Windows** 10+ (amd64, arm64)

For virtual microphone:
- **macOS**: [BlackHole](https://github.com/ExistentialAudio/BlackHole) audio driver
- **Linux**: PulseAudio (`pactl`)

## Network & Firewall

### Server — ports to open

| Port | Protocol | Purpose |
|------|----------|---------|
| `4415` | **TCP** | Signaling (authentication + WebRTC handshake) |
| `4415` | **UDP** | Audio media (WebRTC). Multiplexed — one port handles all clients |
| `4416` | **TCP** | Session info probe (client auto-configuration) |

> Port `4415` is the default and can be changed with `--port`. The session info port is always `port + 1`.

**In short:** open **TCP 4415–4416** and **UDP 4415** inbound on the server.

### Client — no inbound ports required

The client only makes outbound connections. No firewall or port-forwarding rules are needed on the client side.

### LAN discovery

If you use mDNS auto-discovery (`Ctrl+F` in setup), allow **UDP multicast on port 5353**. This can be disabled with `--no-discovery`.

### NAT traversal (STUN / TURN)

EchoWarp uses STUN servers to establish connections through NAT. If both sides are behind symmetric NAT and a direct connection cannot be established, a TURN relay server can be configured (`turn_servers` in config). Both STUN and TURN are **outbound** connections and do not require any inbound firewall rules.

## License

[MIT](LICENSE) — see [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) for third-party licenses.
