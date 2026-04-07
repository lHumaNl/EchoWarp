<p align="center">
  <img src="../../build/icons/icon_512.png" alt="EchoWarp Logo" width="128">
</p>

<h1 align="center">EchoWarp</h1>

<p align="center">
  在主机之间实时传输网络音频
</p>

<p align="center">
  <a href="../../README.md">English</a> |
  <a href="README.ru.md">Русский</a> |
  <a href="README.zh.md">简体中文</a> |
  <a href="README.es.md">Español</a> |
  <a href="README.fr.md">Français</a> |
  <a href="README.it.md">Italiano</a> |
  <a href="README.ko.md">한국어</a> |
  <a href="README.ja.md">日本語</a> |
  <a href="README.de.md">Deutsch</a> |
  <a href="README.pt.md">Português</a> |
  <a href="README.tr.md">Türkçe</a> |
  <a href="README.vi.md">Tiếng Việt</a> |
  <a href="README.pl.md">Polski</a>
</p>

<p align="center">
  <a href="https://github.com/lHumaNl/EchoWarp/releases"><img src="https://img.shields.io/github/v/release/lHumaNl/EchoWarp?style=flat-square" alt="Release"></a>
  <a href="https://github.com/lHumaNl/EchoWarp/actions"><img src="https://img.shields.io/github/actions/workflow/status/lHumaNl/EchoWarp/ci.yml?branch=master&style=flat-square" alt="CI"></a>
  <a href="https://github.com/lHumaNl/EchoWarp/blob/master/LICENSE"><img src="https://img.shields.io/github/license/lHumaNl/EchoWarp?style=flat-square" alt="License"></a>
  <a href="https://github.com/lHumaNl/EchoWarp/releases"><img src="https://img.shields.io/github/downloads/lHumaNl/EchoWarp/total?style=flat-square" alt="Downloads"></a>
</p>

---

在一台机器上采集音频，在另一台机器上实时播放——通过网络传输。EchoWarp 使用 WebRTC 作为传输层，Opus 进行压缩，提供低延迟的端到端加密音频传输。

## 功能特性

- **1:1 流传输** — 服务端采集，客户端播放（或反向）
- **双工模式** — 双向音频，双方均可互相听到
- **广播（1:N）** — 一个服务端向多个客户端推流
- **会议（N:N）** — 多参与者混音，每人获得个人混音（全体减去自身）
- **虚拟麦克风** — 接收到的音频以麦克风输入形式呈现（Discord、Zoom、OBS）
- **局域网发现** — 通过 mDNS 自动检测服务端
- **录音** — WAV 录音：混音轨道、单独轨道，或二者同时
- **SIMD 加速** — 使用 AVX/SSE（x86）和 NEON（ARM）进行音频混音
- **端到端加密** — AES + DTLS/SRTP
- **跨平台** — macOS、Linux、Windows

## 快速开始

1. 从 [Releases](https://github.com/lHumaNl/EchoWarp/releases) 下载预编译二进制文件（无需依赖——opus 已静态链接）
2. 运行 `EchoWarp` — 交互式菜单可选择服务端、客户端、诊断等模式。也可直接指定模式：`EchoWarp server` / `EchoWarp client`
3. 在交互式 TUI 中完成配置，按回车键开始

> **Windows 用户：** 发布包中包含 `EchoWarp Server.bat` 和 `EchoWarp Client.bat`——双击您需要的即可。无需手动打开 cmd.exe。

## 交互式 TUI

TUI 是 EchoWarp 的主要操作界面。只需运行 `EchoWarp server` 或 `EchoWarp client`，交互式配置界面会自动打开。

### 配置界面

配置界面采用双列布局：

**左列 — 音频设备。** 列出所有可用的输入/输出设备及其属性（声道数、采样率、位深）。在双工或会议模式下，可分别为设备分配采集 `[C]` 和播放 `[P]` 角色。虚拟设备的创建可直接在列表中完成。

**右列 — 设置。** 所有会话参数均可在此配置，并支持实时验证：

| 设置 | 说明 |
|---------|-------------|
| 服务器地址 | 服务器 IP/主机名（仅限客户端） |
| 端口 | TCP 端口（默认：4415） |
| 密码 | 认证密码 |
| 模式 | 普通 / 反向 / 双工 |
| 最大客户端数 | 最大连接数（仅限服务端） |
| 采样率 | 8000 / 16000 / 24000 / 48000 Hz |
| 声道 | 单声道 / 立体声 |
| Opus 码率 | 16–512 kbps |
| 回声消除 | 双工模式下的 AEC |
| TLS | 启用/禁用，并配置证书和密钥路径 |
| 日志级别 | debug / info / warn / error |

使用 `Tab` 在列之间切换，用方向键在字段间移动，按 `Enter` 确认并开始推流。

### 局域网发现

在客户端配置界面，按 `Ctrl+F` 打开发现浮层。EchoWarp 使用 mDNS 自动查找本地网络中的服务器——从列表中选择一项，地址和端口字段将自动填写。

### 设备配置文件

按 `Ctrl+O` 打开配置文件浮层。可将当前设备配置保存为命名配置文件、加载之前保存的配置文件，或删除不再需要的配置文件。配置文件会记住设备分配和角色。

### 推流界面

连接成功后，TUI 将显示实时推流状态：

- **连接统计** — RTT、抖动、丢包，以及显示与质量上限接近程度的阈值条
- **连接质量** — 由 RTT、抖动和丢包综合计算的总体指标（极佳 / 良好 / 一般 / 差）
- **频谱分析仪** — 音频信号的实时 FFT 频率可视化
- **VU 电平表** — 每声道音频电平显示
- **码率** — 实时上行/下行码率显示
- **设备控制** — 音量调节、静音、设备切换
- **日志面板** — 可开关的滚动日志视图

### TUI 快捷键

| 按键 | 操作 |
|-----|--------|
| `Tab` | 切换列（配置界面） |
| `Ctrl+F` | 局域网发现（客户端配置） |
| `Ctrl+O` | 设备配置文件（配置界面） |
| `Enter` | 确认并开始 |
| `Ctrl+Q` | 退出 |
| `Ctrl+P` | 暂停/恢复推流 |
| `Ctrl+L` | 切换日志面板 |
| `Ctrl+M` | 静音/取消静音 |
| `+` / `-` | 增大/减小音量 |
| `Ctrl+Up/Down` | 滚动日志 |
| `Ctrl+R` | 开始/停止录音 |
| `Ctrl+D` | 踢出客户端（服务端） |
| `Ctrl+B` | 封禁客户端（服务端） |

## 流传输模式

EchoWarp 有四种流传输模式。根据您的场景选择合适的模式——模式在 TUI 配置界面（服务端）中选择，或由客户端自动检测。

### 普通 — 单向：服务端到客户端

服务端采集音频并发送给所有已连接的客户端。客户端仅接收音频。

```
Server [采集音频] ───→ Client 1 [播放音频]
                  ├──→ Client 2 [播放音频]
                  └──→ Client N [播放音频]
```

**适用场景：** 向听众推送音乐/播客，将系统音频（YouTube、Spotify）广播到另一个房间，将回环音频源发送到远端机器。

**所需设备：**

| 端 | 采集（输入） | 播放（输出） |
|------|:-:|:-:|
| 服务端 | 需要 | — |
| 客户端 | — | 需要 |

**参与者：** 1 个服务端 + 1…N 个客户端（在 TUI 中设置*最大客户端数*）。

---

### 反向 — 单向：客户端到服务端

与普通模式相反。客户端采集音频并发送给服务端。服务端仅接收音频。

```
Client 1 [采集音频] ───→ Server [播放音频]
Client 2 [采集音频] ──┘
```

**适用场景：** 在服务端机器上使用远端麦克风，从远端位置采集音频，将客户端的麦克风发送到服务端扬声器。

**所需设备：**

| 端 | 采集（输入） | 播放（输出） |
|------|:-:|:-:|
| 服务端 | — | 需要 |
| 客户端 | 需要 | — |

**参与者：** 1 个服务端 + 1…N 个客户端。

---

### 双工 — 双向

双方同时采集和播放。所有人都能听到彼此。

```
Server [麦克风 + 扬声器] ⟷ Client 1 [麦克风 + 扬声器]
                         ⟷ Client 2 [麦克风 + 扬声器]
```

**适用场景：** 两台或多台机器之间的语音通话，房间之间的对讲，协作音频会话。

**所需设备：**

| 端 | 采集（输入） | 播放（输出） |
|------|:-:|:-:|
| 服务端 | 需要 | 需要 |
| 客户端 | 需要 | 需要 |

**参与者：** 1 个服务端 + 1…N 个客户端。支持 AEC（回声消除）以防止反馈回路。

---

### 会议 — 多用户混音（N:N）

服务端充当混音集线器。每个参与者（服务端 + 客户端）发送自己的音频，并接收一个**除自身外所有人**的个人混音（全体减去自身）。这样可以避免听到自己的声音。

```
Client 1 ──┐              ┌──→ Client 1 (听到 Server + 2 + 3)
Client 2 ──┤  Server hub  ├──→ Client 2 (听到 Server + 1 + 3)
Client 3 ──┘              └──→ Client 3 (听到 Server + 1 + 2)
```

**适用场景：** 3台及以上机器的群组通话，远程排练，多参与者会议。

**所需设备：**

| 端 | 采集（输入） | 播放（输出） |
|------|:-:|:-:|
| 服务端（仅集线器） | — | — |
| 服务端（参与者） | 需要 | 需要 |
| 客户端 | 需要 | 需要 |

服务端可以作为**纯集线器**运行（无本地音频——仅为客户端混音），也可以作为拥有自己麦克风和扬声器的完整参与者。

**参与者：** 1 个服务端 + 2…N 个客户端。支持录音（混音、分轨或二者同时）、踢出/封禁，以及逐参与者音量控制。

---

### 模式总览

| | 普通 | 反向 | 双工 | 会议 |
|---|---|---|---|---|
| **方向** | 服务端 → 客户端 | 客户端 → 服务端 | 双向 | 所有人 ⟷ 所有人 |
| **服务端设备** | 仅采集 | 仅播放 | 两者 | 两者（纯集线器则无需） |
| **客户端设备** | 仅播放 | 仅采集 | 两者 | 两者 |
| **最大参与者数** | 1 + N | 1 + N | 1 + N | 1 + N |
| **AEC 支持** | — | — | 是 | 是 |
| **个人混音** | — | — | — | 是 |
| **录音** | — | — | — | 是 |
| **CLI 参数** | *（默认）* | `-r` | `-X` | `--conference` |

> **提示：** 客户端数量由 TUI 中的*最大客户端数*设置控制（默认：1）。如需广播或多用户场景，请将其调高。

## 音频路由指南

### 使用场景

EchoWarp 支持多种音频路由方案。下表展示了每种场景应使用的模式和设备配置：

| 场景 | 模式 | 服务端设备 | 客户端设备 |
|------|------|-----------|-----------|
| 将麦克风流传到远端扬声器 | 普通 | 麦克风 `[输入]` | 扬声器 `[输出]` |
| 流传系统音频（YouTube、Spotify） | 普通 | 🔄 回环 `[输入]` | 扬声器 `[输出]` |
| 在 Discord/Zoom 中使用远端麦克风 | 反向 | 扬声器 `[输出]` | 麦克风 `[输入]` |
| 双向语音通话 | 双工 | 麦克风 + 扬声器 | 麦克风 + 扬声器 |
| 群组通话（3台及以上设备） | 会议 | —（集线器） | 麦克风 + 扬声器 |
| 在 OBS 中将远端音频作为虚拟麦克风 | 普通 | 🔄 回环 `[输入]` | ⟡ 虚拟麦克风 `[输出]` |
| 将流和本地麦克风合为一个虚拟麦克风 | 普通 | 麦克风 `[输入]` | ⟡ 虚拟 `[输出]` + 🎤 混音麦克风 |

### 回环 — 捕获系统音频

回环设备可捕获计算机上播放的所有音频（音乐、视频通话、游戏音效），并将其作为输入源提供使用。

| 平台 | 工作原理 | 设置方法 |
|------|---------|---------|
| **macOS** | 聚合设备（扬声器 + BlackHole） | 安装 [BlackHole](https://github.com/ExistentialAudio/BlackHole)：`brew install blackhole-2ch`。回环设备会自动出现在 TUI 中 |
| **Windows** | WASAPI 原生回环 | 内置支持，无需额外软件 |
| **Linux** | PulseAudio 监听源 | PulseAudio/PipeWire 内置支持 |

在 TUI 中，回环设备以 🔄 标记，显示在输入（Input）区域。

### 虚拟麦克风 — 将音频路由到其他应用

虚拟麦克风使接收到的音频以麦克风输入的形式出现，供 Discord、Zoom、OBS 及其他应用程序使用。

| 平台 | 驱动程序 | 安装方式 |
|------|---------|---------|
| **macOS** | [BlackHole](https://github.com/ExistentialAudio/BlackHole) | `brew install blackhole-2ch` |
| **Windows** | [VB-Audio Virtual Cable](https://vb-audio.com/Cable/) | 从 vb-audio.com 下载 |
| **Linux** | PulseAudio 空输出槽 | 在 TUI 中创建：设置 → 虚拟麦克风 → 创建 |

在 macOS 和 Windows 上，安装驱动后在 TUI 的输出（Output）区域选择该设备。在 Linux 上，TUI 会自动创建虚拟输出槽——在其他应用中，选择 **"Monitor of EchoWarp"** 作为麦克风。

虚拟设备在 TUI 中以 ⟡ 标记，并显示 **adaptive**（自适应）而非固定采样率。

### 将本地麦克风混入虚拟输出

当您将远端音频流路由到虚拟输出设备（BlackHole、VB-Cable）时，第三方应用（如 Discord 或 Zoom）只能接收到该音频流——它们无法听到您的本地麦克风。如果您需要让自己的声音**和**音频流同时通过一个虚拟麦克风输入呈现，EchoWarp 可以将二者混合在一起。

**使用方法：** 在 TUI 中，于输出（Output）区域选择一个虚拟设备。其下方会以子项的形式显示输入设备——勾选您想要混入的麦克风：

```
🔊 输出
  ⟡ BlackHole 2ch                           stereo 32-bit adaptive   [✓]
      └─ 🎤 内置麦克风                                                [✓]
```

混合后的音频（音频流 + 麦克风）将被写入虚拟输出。在 Discord/Zoom/OBS 中，选择该虚拟设备作为麦克风——对方将同时听到音频流和您的声音。

**操作系统级替代方案**（不使用内置混音器）：

| 平台 | 方法 | 详情 |
|------|------|------|
| **macOS** | 聚合设备 | 打开"音频 MIDI 设置" → 创建一个聚合设备，组合您的麦克风和 BlackHole。在应用中选择该聚合设备作为输入 |
| **Windows** | [VoiceMeeter](https://vb-audio.com/Voicemeeter/) | 免费虚拟混音器——将 VB-Cable 和您的麦克风路由到 VoiceMeeter，然后在应用中使用其输出作为麦克风 |
| **Linux** | PulseAudio combine-source | `pactl load-module module-combine-source source_name=combined slaves=<mic>,<virtual>.monitor` |

内置混音器适用于所有平台，无需任何额外配置。

### 设备区域

TUI 仅显示与当前模式相关的区域：

| 模式 + 端 | 可见区域 |
|---------|---------|
| 普通服务端 / 反向客户端 | 仅 🎤 输入 |
| 普通客户端 / 反向服务端 | 仅 🔊 输出 |
| 双工 / 会议 | 🎤 输入 + 🔊 输出 |

### 诊断

运行 `EchoWarp doctor` 检查音频配置：

```
Audio
[OK] Audio system: 2 input device(s), 2 output device(s)
[OK] Virtual audio: BlackHole 2ch detected
```

如果未找到虚拟音频驱动，doctor 命令将显示适用于当前平台的安装说明。

### 常见问题

**我想在另一个房间的扬声器上收听本机播放的音乐。**
→ 在播放音乐的机器上运行 `EchoWarp server`，在输入（Input）中选择 🔄 回环设备（即您的扬声器）。在远端机器上运行 `EchoWarp client`，在输出（Output）中选择扬声器。

**我想在 Zoom/Discord 中使用远端麦克风。**
→ 在远端机器（有麦克风的机器）上以反向（Reverse）模式运行 `EchoWarp server`。在本机上运行 `EchoWarp client`，在输出（Output）中选择虚拟音频设备（BlackHole/VB-Cable）。在 Zoom/Discord 中，将该虚拟设备选为麦克风。

**我想将系统音频（YouTube、Spotify）流传到另一台机器。**
→ 与"在另一个房间播放音乐"相同——在服务端使用 🔄 回环设备。回环设备会捕获通过所选扬声器播放的所有内容。

**我想让远端音频在 OBS 中显示为麦克风输入。**
→ 在有音频源的机器上运行服务端。在本机（客户端），在输出（Output）中选择虚拟音频设备。在 OBS 中，添加"音频输入采集"来源并选择该虚拟设备。

**我想在两台电脑之间进行双向语音通话。**
→ 使用双工（Duplex）模式。在两台机器上分别在输入（Input）中选择麦克风，在输出（Output）中选择扬声器。

**我想进行3台及以上设备的群组通话。**
→ 使用会议（Conference）模式。服务端充当集线器（无需配置设备）。每个客户端选择麦克风和扬声器。所有人可以听到彼此的声音，但不会听到自己的回声。

**我想让 Discord 通过一个虚拟麦克风同时听到远端音频流和我的声音。**
→ 在客户端，于输出（Output）中选择一个虚拟设备（BlackHole/VB-Cable）。其下方会显示输入设备列表——勾选您的麦克风。EchoWarp 会将音频流和您的麦克风混合后写入虚拟输出。在 Discord 中，选择该虚拟设备作为麦克风即可。

**我没有安装 BlackHole / VB-Cable。**
→ 运行 `EchoWarp doctor`——它会告诉您需要安装什么以及如何安装。在 Linux 上，无需额外软件。

**TUI 中没有显示回环设备。**
→ macOS：请先安装 BlackHole（`brew install blackhole-2ch`）。Windows/Linux：回环设备为内置支持，应会自动显示。

**虚拟设备显示"adaptive"而非采样率。**
→ 这是正常现象。虚拟音频驱动（BlackHole、VB-Cable）会自适应应用程序所使用的采样率——显示的采样率没有实际意义。

<details>
<summary>macOS："无法打开 EchoWarp" / Gatekeeper 警告</summary>

macOS 会阻止未签名的应用程序。要允许 EchoWarp 运行：

```bash
xattr -cr /path/to/EchoWarp       # 针对二进制文件
xattr -cr /path/to/EchoWarp.app   # 针对 .app 包
```

或者：**系统设置 → 隐私与安全性 → "仍要打开"**

</details>

## CLI 模式

TUI 中所有可用的设置也可以通过 CLI 参数传入，适用于脚本和自动化场景：

```bash
# 服务端
EchoWarp server -d 1 -P mypassword

# 客户端（如果省略 -a，则自动在局域网中发现服务器）
EchoWarp client -a 192.168.1.10 -d 2 -P mypassword

# 列出音频设备
EchoWarp devices
```

### 模式

各模式的详细说明请参阅[流传输模式](#流传输模式)。

| 模式 | 参数 |
|------|------|
| 普通 | *（默认）* |
| 反向 | `-r` |
| 双工 | `-X` |
| 会议 | `--conference` |

### 常用参数

| 参数 | 缩写 | 说明 |
|------|-------|-------------|
| `--device` | `-d` | 音频设备 ID |
| `--device-name` | `-D` | 按名称选择设备（子字符串匹配） |
| `--password` | `-P` | 认证密码 |
| `--port` | `-p` | TCP 端口（默认：4415） |
| `--sample-rate` | | 采样率（默认：48000） |
| `--channels` | | 1=单声道，2=立体声（默认：1） |
| `--max-clients` | | 最大连接客户端数（默认：1） |
| `--virtual-mic` | | 创建虚拟麦克风设备 |
| `--config` | `-c` | 加载 YAML 配置文件 |
| `--save-config` | `-s` | 将当前设置保存为 YAML |
| `--profile` | | 加载已保存的设备配置文件 |
| `--stun-server` | | 用于 NAT 穿透的自定义 STUN 服务器 |
| `--tls-cert` / `--tls-key` | | TLS 证书和密钥 |
| `--log-level` | | debug、info、warn、error |
| `--aec` | | 声学回声消除（双工模式） |
| `--loopback` | | 采集系统音频（macOS，需要 BlackHole） |
| `--dry-run` | | 验证配置后退出 |

### 配置文件

设置优先级：CLI 参数 > 环境变量（`ECHOWARP_*`）> 配置文件 > 默认值。

TUI 设置界面支持交互式保存和加载配置文件。也可以使用 CLI 参数：

```bash
# 将当前设置保存到文件
EchoWarp server -d 1 -P pass --channels 2 -s myconfig.yml

# 从文件加载设置
EchoWarp server -c myconfig.yml
```

## 安装

### 预编译二进制文件

从 [Releases](https://github.com/lHumaNl/EchoWarp/releases) 页面下载，支持以下平台：
- **macOS** — arm64（Apple Silicon）、amd64（Intel），包含 `.app` 应用包
- **Linux** — amd64, arm64, armv7, armv6
- **Windows** — amd64, arm64

### 从源码构建

需要 Go 1.22+ 及 libopus 开发头文件。

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

生成的二进制文件静态链接了 opus，无需运行时依赖。

## 系统要求

- **macOS** 11.0+（arm64、amd64）
- **Linux**（amd64、arm64、armv7、armv6）
- **Windows** 10+（amd64、arm64）

虚拟麦克风需要：
- **macOS**：[BlackHole](https://github.com/ExistentialAudio/BlackHole) 音频驱动
- **Windows**：[VB-Audio Virtual Cable](https://vb-audio.com/Cable/)
- **Linux**：PulseAudio（`pactl`）

## 网络与防火墙

### 服务端 — 需要开放的端口

| 端口 | 协议 | 用途 |
|------|------|------|
| `4415` | **TCP** | 信令（认证 + WebRTC 握手） |
| `4415` | **UDP** | 音频媒体（WebRTC）。多路复用 — 单端口处理所有客户端 |
| `4416` | **TCP** | 会话信息探测（客户端自动配置） |

> 端口 `4415` 为默认值，可通过 `--port` 更改。会话信息端口始终为 `port + 1`。

**简而言之：** 在服务端入站方向开放 **TCP 4415–4416** 和 **UDP 4415**。

### 客户端 — 无需开放入站端口

客户端仅发起出站连接，无需在客户端配置任何防火墙或端口转发规则。

### 局域网发现

如果使用 mDNS 自动发现功能（设置中按 `Ctrl+F`），请允许 **UDP 组播端口 5353**。可通过 `--no-discovery` 禁用此功能。

### NAT 穿透（STUN / TURN）

EchoWarp 使用 STUN 服务器通过 NAT 建立连接。如果双方都位于对称型 NAT 之后且无法建立直接连接，可配置 TURN 中继服务器（配置文件中的 `turn_servers`）。STUN 和 TURN 均为**出站**连接，不需要任何入站防火墙规则。

## 许可证

[MIT](../../LICENSE) — 第三方许可证请参阅 [THIRD_PARTY_NOTICES.md](../../THIRD_PARTY_NOTICES.md)。
