<p align="center">
  <img src="../../build/icons/icon_512.png" alt="EchoWarp Logo" width="128">
</p>

<h1 align="center">EchoWarp</h1>

<p align="center">
  호스트 간 실시간 네트워크 오디오 스트리밍
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
  <a href="https://github.com/lHumaNl/EchoWarp/actions"><img src="https://img.shields.io/github/actions/workflow/status/lHumaNl/EchoWarp/ci.yml?branch=main&style=flat-square" alt="CI"></a>
  <a href="https://github.com/lHumaNl/EchoWarp/blob/master/LICENSE"><img src="https://img.shields.io/github/license/lHumaNl/EchoWarp?style=flat-square" alt="License"></a>
  <a href="https://github.com/lHumaNl/EchoWarp/releases"><img src="https://img.shields.io/github/downloads/lHumaNl/EchoWarp/total?style=flat-square" alt="Downloads"></a>
</p>

---

한 머신에서 오디오를 캡처하여 다른 머신에서 실시간으로 재생하세요 — 네트워크를 통해. EchoWarp는 전송에 WebRTC, 압축에 Opus를 사용하며, 종단간 암호화와 함께 저지연 오디오를 제공합니다.

## 기능

- **1:1 스트리밍** — 서버가 캡처하고 클라이언트가 재생 (또는 역방향)
- **듀플렉스 모드** — 양방향 오디오, 양측이 서로의 소리를 들을 수 있음
- **브로드캐스트 (1:N)** — 하나의 서버가 여러 클라이언트에 스트리밍
- **컨퍼런스 (N:N)** — 개인 믹스를 지원하는 다참여자 믹싱 (전체 마이너스 자신)
- **가상 마이크** — 수신 오디오가 마이크 입력으로 나타남 (Discord, Zoom, OBS)
- **LAN 탐색** — mDNS를 통한 서버 자동 감지
- **녹음** — WAV 녹음: 믹스, 트랙별, 또는 둘 다
- **SIMD 가속** — 오디오 믹싱을 위한 AVX/SSE (x86) 및 NEON (ARM)
- **종단간 암호화** — AES + DTLS/SRTP
- **크로스 플랫폼** — macOS, Linux, Windows

## 빠른 시작

1. [Releases](https://github.com/lHumaNl/EchoWarp/releases)에서 미리 빌드된 바이너리를 다운로드 (의존성 없음 — opus는 정적으로 링크됨)
2. `EchoWarp` 실행 — 대화형 메뉴에서 서버, 클라이언트, 진단 등을 선택할 수 있습니다. 또는 `EchoWarp server` / `EchoWarp client`로 바로 모드 진입
3. 대화형 TUI에서 모든 설정을 구성하고 Enter를 눌러 시작

> **Windows 사용자:** 릴리스 아카이브에는 `EchoWarp Server.bat` 및 `EchoWarp Client.bat`이 포함되어 있습니다 — 필요한 것을 더블 클릭하세요. cmd.exe를 수동으로 열 필요가 없습니다.

## 대화형 TUI

TUI는 EchoWarp의 기본 인터페이스입니다. `EchoWarp server` 또는 `EchoWarp client`를 실행하면 대화형 설정 화면이 자동으로 열립니다.

### 설정 화면

설정 화면은 두 열 레이아웃으로 구성됩니다:

**왼쪽 열 — 오디오 장치.** 모든 사용 가능한 입력/출력 장치와 해당 속성(채널, 샘플 레이트, 비트 심도)을 나열합니다. 듀플렉스 또는 컨퍼런스 모드에서는 별도의 캡처 `[C]` 및 재생 `[P]` 역할을 할당합니다. 가상 장치 생성은 목록에서 직접 사용할 수 있습니다.

**오른쪽 열 — 설정.** 모든 세션 매개변수는 인라인 유효성 검사와 함께 여기서 구성할 수 있습니다:

| 설정 | 설명 |
|------|------|
| 서버 주소 | 서버 IP/호스트명 (클라이언트만 해당) |
| 포트 | TCP 포트 (기본값: 4415) |
| 비밀번호 | 인증 비밀번호 |
| 모드 | 일반 / 역방향 / 듀플렉스 |
| 최대 클라이언트 | 최대 연결 수 (서버만 해당) |
| 샘플 레이트 | 8000 / 16000 / 24000 / 48000 Hz |
| 채널 | 모노 / 스테레오 |
| Opus 비트레이트 | 16–512 kbps |
| 에코 제거 | 듀플렉스 모드를 위한 AEC |
| TLS | 인증서 및 키 경로와 함께 활성화/비활성화 |
| 로그 레벨 | debug / info / warn / error |

`Tab`으로 열 사이를 이동하고, 화살표 키로 필드를 이동하며, `Enter`를 눌러 확인하고 스트리밍을 시작합니다.

### LAN 탐색

클라이언트 설정 화면에서 `Ctrl+F`를 눌러 탐색 오버레이를 엽니다. EchoWarp는 mDNS를 사용하여 로컬 네트워크에서 서버를 자동으로 찾습니다 — 목록에서 하나를 선택하면 주소/포트 필드가 자동으로 채워집니다.

### 장치 프로필

`Ctrl+O`를 눌러 프로필 오버레이를 엽니다. 현재 장치 구성을 이름 있는 프로필로 저장하거나, 이전에 저장한 프로필을 불러오거나, 더 이상 필요하지 않은 프로필을 삭제합니다. 프로필은 장치 할당과 역할을 기억합니다.

### 스트리밍 화면

연결되면 TUI는 실시간 스트리밍 상태를 표시합니다:

- **연결 통계** — RTT, 지터, 패킷 손실과 품질 한계에 대한 근접도를 나타내는 임계값 막대
- **연결 품질** — RTT, 지터, 손실에서 계산된 전반적인 지표 (탁월 / 양호 / 보통 / 불량)
- **스펙트럼 분석기** — 오디오 신호의 실시간 FFT 주파수 시각화
- **VU 미터** — 채널별 오디오 레벨 미터
- **비트레이트** — 실시간 업로드/다운로드 비트레이트 표시
- **장치 제어** — 볼륨 조절, 음소거, 장치 전환
- **로그 패널** — 토글 가능한 스크롤 가능한 로그 뷰

### TUI 키보드 단축키

| 키 | 동작 |
|----|------|
| `Tab` | 열 전환 (설정) |
| `Ctrl+F` | LAN 탐색 (클라이언트 설정) |
| `Ctrl+O` | 장치 프로필 (설정) |
| `Enter` | 확인 및 시작 |
| `Ctrl+Q` | 종료 |
| `Ctrl+P` | 스트리밍 일시 중지/재개 |
| `Ctrl+L` | 로그 패널 토글 |
| `Ctrl+M` | 음소거/음소거 해제 |
| `+` / `-` | 볼륨 올리기/내리기 |
| `Ctrl+Up/Down` | 로그 스크롤 |
| `Ctrl+R` | 녹음 시작/중지 |
| `Ctrl+D` | 클라이언트 강제 퇴장 (서버) |
| `Ctrl+B` | 클라이언트 차단 (서버) |

## 스트리밍 모드

EchoWarp에는 네 가지 스트리밍 모드가 있습니다. 시나리오에 맞는 모드를 선택하세요 — 모드는 TUI 설정 화면(서버 측)에서 선택하거나 자동으로 감지(클라이언트 측)됩니다.

### 일반 — 단방향: 서버에서 클라이언트로

서버가 오디오를 캡처하여 연결된 모든 클라이언트에 전송합니다. 클라이언트는 수신만 합니다.

```
Server [captures audio] ───→ Client 1 [plays audio]
                         ├──→ Client 2 [plays audio]
                         └──→ Client N [plays audio]
```

**사용 시기:** 리스너에게 음악/팟캐스트를 스트리밍, 시스템 오디오(YouTube, Spotify)를 다른 방으로 브로드캐스트, 루프백 피드를 원격 기기로 전송.

**필요한 장치:**

| 역할 | 캡처 (입력) | 재생 (출력) |
|------|:-:|:-:|
| 서버 | 필수 | — |
| 클라이언트 | — | 필수 |

**참가자:** 서버 1대 + 클라이언트 1…N대 (TUI에서 *최대 클라이언트* 설정).

---

### 역방향 — 단방향: 클라이언트에서 서버로

일반 모드의 반대입니다. 클라이언트가 오디오를 캡처하여 서버로 전송합니다. 서버는 수신만 합니다.

```
Client 1 [captures audio] ───→ Server [plays audio]
Client 2 [captures audio] ──┘
```

**사용 시기:** 서버 기기에서 원격 마이크 사용, 원격 위치에서 오디오 수집, 클라이언트의 마이크를 서버 스피커로 전송.

**필요한 장치:**

| 역할 | 캡처 (입력) | 재생 (출력) |
|------|:-:|:-:|
| 서버 | — | 필수 |
| 클라이언트 | 필수 | — |

**참가자:** 서버 1대 + 클라이언트 1…N대.

---

### 듀플렉스 — 양방향

양쪽 모두 동시에 캡처하고 재생합니다. 모든 참가자가 서로의 소리를 들을 수 있습니다.

```
Server [mic + speakers] ⟷ Client 1 [mic + speakers]
                        ⟷ Client 2 [mic + speakers]
```

**사용 시기:** 두 대 이상의 기기 간 음성 통화, 방 사이 인터콤, 협업 오디오 세션.

**필요한 장치:**

| 역할 | 캡처 (입력) | 재생 (출력) |
|------|:-:|:-:|
| 서버 | 필수 | 필수 |
| 클라이언트 | 필수 | 필수 |

**참가자:** 서버 1대 + 클라이언트 1…N대. 피드백 루프 방지를 위한 AEC(에코 제거)를 지원합니다.

---

### 컨퍼런스 — 다중 사용자 믹싱 (N:N)

서버가 믹싱 허브 역할을 합니다. 각 참가자(서버 + 클라이언트)는 자신의 오디오를 전송하고 **다른 모든 사람**(전체 마이너스 자신)의 개인 믹스를 수신합니다. 이를 통해 자신의 목소리가 되돌아오는 것을 방지합니다.

```
Client 1 ──┐              ┌──→ Client 1 (hears Server + 2 + 3)
Client 2 ──┤  Server hub  ├──→ Client 2 (hears Server + 1 + 3)
Client 3 ──┘              └──→ Client 3 (hears Server + 1 + 2)
```

**사용 시기:** 3대 이상의 기기로 그룹 통화, 원격 리허설, 다참여자 컨퍼런스.

**필요한 장치:**

| 역할 | 캡처 (입력) | 재생 (출력) |
|------|:-:|:-:|
| 서버 (허브 전용) | — | — |
| 서버 (참가자) | 필수 | 필수 |
| 클라이언트 | 필수 | 필수 |

서버는 **허브 전용**(로컬 오디오 없이 클라이언트를 위한 믹싱만 수행)으로 작동하거나 자체 마이크와 스피커를 갖춘 전체 참가자로 작동할 수 있습니다.

**참가자:** 서버 1대 + 클라이언트 2…N대. 녹음(믹스, 트랙별, 또는 둘 다), 강제 퇴장/차단, 참가자별 볼륨 제어를 지원합니다.

---

### 모드 요약

| | 일반 | 역방향 | 듀플렉스 | 컨퍼런스 |
|---|---|---|---|---|
| **방향** | 서버 → 클라이언트 | 클라이언트 → 서버 | 양방향 | 모든 참가자 ⟷ 모든 참가자 |
| **서버 장치** | 캡처만 | 재생만 | 둘 다 | 둘 다 (허브인 경우 없음) |
| **클라이언트 장치** | 재생만 | 캡처만 | 둘 다 | 둘 다 |
| **최대 참가자** | 1 + N | 1 + N | 1 + N | 1 + N |
| **AEC 지원** | — | — | 예 | 예 |
| **개인 믹스** | — | — | — | 예 |
| **녹음** | — | — | — | 예 |
| **CLI 플래그** | *(기본값)* | `-r` | `-X` | `--conference` |

> **팁:** 클라이언트 수는 TUI의 *최대 클라이언트* 설정으로 제어됩니다 (기본값: 1). 브로드캐스트 또는 다중 사용자 시나리오가 필요하면 값을 높이세요.

## 오디오 라우팅 가이드

### 사용 사례

EchoWarp는 여러 가지 오디오 라우팅 시나리오를 지원합니다. 아래 표는 각 시나리오에 맞는 모드 및 장치 설정을 보여줍니다:

| 시나리오 | 모드 | 서버 장치 | 클라이언트 장치 |
|----------|------|-----------|----------------|
| 마이크를 원격 스피커로 스트리밍 | Normal | 마이크 `[Input]` | 스피커 `[Output]` |
| 시스템 오디오 스트리밍 (YouTube, Spotify) | Normal | 🔄 루프백 `[Input]` | 스피커 `[Output]` |
| Discord/Zoom에서 원격 마이크 사용 | Reverse | 스피커 `[Output]` | 마이크 `[Input]` |
| 양방향 음성 채팅 | Duplex | 마이크 + 스피커 | 마이크 + 스피커 |
| 그룹 통화 (3대 이상) | Conference | — (허브) | 마이크 + 스피커 |
| OBS에서 원격 오디오를 가상 마이크로 사용 | Normal | 🔄 루프백 `[Input]` | ⟡ 가상 마이크 `[Output]` |
| 스트림 + 로컬 마이크를 하나의 가상 마이크로 | Normal | 마이크 `[Input]` | ⟡ 가상 `[Output]` + 🎤 믹스 마이크 |

### 루프백 — 시스템 오디오 캡처

루프백 장치는 기기에서 재생 중인 모든 오디오(음악, 화상 통화, 게임 소리)를 캡처하여 입력 소스로 사용할 수 있게 합니다.

| 플랫폼 | 작동 방식 | 설정 |
|--------|----------|------|
| **macOS** | 집합 장치 (스피커 + BlackHole) | [BlackHole](https://github.com/ExistentialAudio/BlackHole) 설치: `brew install blackhole-2ch`. 루프백 장치는 TUI에 자동으로 표시됩니다 |
| **Windows** | WASAPI 네이티브 루프백 | 기본 내장, 추가 소프트웨어 불필요 |
| **Linux** | PulseAudio 모니터 소스 | PulseAudio/PipeWire에 기본 내장 |

TUI에서 루프백 장치는 🔄 로 표시되며 Input 섹션에 나타납니다.

### 가상 마이크 — 다른 앱으로 오디오 라우팅

가상 마이크를 사용하면 수신된 오디오가 Discord, Zoom, OBS 등의 애플리케이션에서 마이크 입력으로 인식됩니다.

| 플랫폼 | 드라이버 | 설치 |
|--------|---------|------|
| **macOS** | [BlackHole](https://github.com/ExistentialAudio/BlackHole) | `brew install blackhole-2ch` |
| **Windows** | [VB-Audio Virtual Cable](https://vb-audio.com/Cable/) | vb-audio.com에서 다운로드 |
| **Linux** | PulseAudio null-sink | TUI에서 생성: 설정 → 가상 마이크 → 생성 |

macOS 및 Windows에서는 드라이버를 설치한 후 TUI의 Output 섹션에서 선택하십시오. Linux에서는 TUI가 가상 싱크를 자동으로 생성합니다 — 다른 앱에서 마이크로 **"Monitor of EchoWarp"** 를 선택하십시오.

가상 장치는 TUI에서 ⟡ 로 표시되며 고정된 샘플 레이트 대신 **adaptive** 로 표시됩니다.

### 로컬 마이크를 가상 출력에 믹싱

원격 스트림을 가상 출력 장치(BlackHole, VB-Cable)로 라우팅하면 Discord나 Zoom 같은 서드파티 앱에는 스트림만 들립니다 — 로컬 마이크 소리는 들리지 않습니다. 내 목소리**와** 스트림을 하나의 마이크 입력으로 사용하려면 EchoWarp에서 두 소스를 함께 믹싱할 수 있습니다.

**사용 방법:** TUI에서 Output 섹션의 가상 장치를 선택하십시오. 그 아래에 입력 장치가 하위 항목으로 나타납니다 — 믹싱할 마이크를 체크하십시오:

```
🔊 Output
  ⟡ BlackHole 2ch                           stereo 32-bit adaptive   [✓]
      └─ 🎤 Built-in Microphone                                      [✓]
```

믹싱된 오디오(스트림 + 마이크)가 가상 출력에 기록됩니다. Discord/Zoom/OBS에서 해당 가상 장치를 마이크로 선택하면 스트림과 내 목소리가 모두 들립니다.

**OS 수준 대안** (내장 믹서를 사용하지 않는 경우):

| 플랫폼 | 방법 | 세부 사항 |
|--------|------|-----------|
| **macOS** | 집합 장치 | Audio MIDI 설정을 열고 → 마이크 + BlackHole을 결합한 집합 장치를 생성합니다. 앱에서 해당 집합 장치를 입력으로 선택하십시오 |
| **Windows** | [VoiceMeeter](https://vb-audio.com/Voicemeeter/) | 무료 가상 믹서 — VB-Cable과 마이크를 VoiceMeeter에 라우팅하고 VoiceMeeter 출력을 앱에서 마이크로 사용하십시오 |
| **Linux** | PulseAudio combine-source | `pactl load-module module-combine-source source_name=combined slaves=<mic>,<virtual>.monitor` |

내장 믹서는 모든 플랫폼에서 작동하며 추가 구성이 필요하지 않습니다.

### 장치 섹션

TUI는 현재 모드에 관련된 섹션만 표시합니다:

| 모드 + 역할 | 표시되는 섹션 |
|------------|--------------|
| Normal 서버 / Reverse 클라이언트 | 🎤 Input만 |
| Normal 클라이언트 / Reverse 서버 | 🔊 Output만 |
| Duplex / Conference | 🎤 Input + 🔊 Output |

### 진단

오디오 설정을 확인하려면 `EchoWarp doctor` 를 실행하십시오:

```
Audio
[OK] Audio system: 2 input device(s), 2 output device(s)
[OK] Virtual audio: BlackHole 2ch detected
```

가상 오디오 드라이버가 없으면 doctor가 해당 플랫폼의 설치 지침을 표시합니다.

### FAQ

**다른 방의 스피커로 내 컴퓨터 음악을 듣고 싶습니다.**
→ 음악 기기에서 `EchoWarp server` 를 실행하고 Input에서 🔄 루프백 장치(스피커)를 선택하십시오. 원격 기기에서 `EchoWarp client` 를 실행하고 Output에서 스피커를 선택하십시오.

**Zoom/Discord에서 원격 마이크를 사용하고 싶습니다.**
→ 마이크가 있는 원격 기기에서 Reverse 모드로 `EchoWarp server` 를 실행하십시오. 내 기기에서 `EchoWarp client` 를 실행하고 Output에서 가상 오디오 장치(BlackHole/VB-Cable)를 선택하십시오. Zoom/Discord에서 해당 가상 장치를 마이크로 선택하십시오.

**시스템 오디오(YouTube, Spotify)를 다른 기기로 스트리밍하고 싶습니다.**
→ "다른 방의 음악"과 동일합니다 — 서버 측에서 🔄 루프백을 사용하십시오. 루프백 장치는 선택한 스피커를 통해 재생되는 모든 것을 캡처합니다.

**원격 오디오가 OBS에서 마이크 입력으로 표시되기를 원합니다.**
→ 오디오 소스와 함께 서버를 실행하십시오. 내 기기(클라이언트)의 Output에서 가상 오디오 장치를 선택하십시오. OBS에서 "오디오 입력 캡처" 소스를 추가하고 가상 장치를 선택하십시오.

**두 컴퓨터 간 양방향 음성 채팅을 원합니다.**
→ Duplex 모드를 사용하십시오. 두 기기 모두 Input에서 마이크를, Output에서 스피커를 선택하십시오.

**3대 이상으로 그룹 통화를 하고 싶습니다.**
→ Conference 모드를 사용하십시오. 서버는 허브 역할을 합니다(장치 불필요). 각 클라이언트는 마이크와 스피커를 선택합니다. 모든 참가자가 자신의 목소리를 제외한 다른 모든 소리를 들을 수 있습니다.

**Discord에서 원격 스트림과 내 목소리를 하나의 가상 마이크로 사용하고 싶습니다.**
→ 클라이언트에서 Output에 가상 장치(BlackHole/VB-Cable)를 선택하십시오. 그 아래에 입력 장치 목록이 나타납니다 — 마이크를 체크하십시오. EchoWarp가 스트림과 마이크를 가상 출력으로 믹싱합니다. Discord에서 해당 가상 장치를 마이크로 선택하십시오.

**BlackHole / VB-Cable이 설치되어 있지 않습니다.**
→ `EchoWarp doctor` 를 실행하십시오 — 정확히 무엇을 어떻게 설치해야 하는지 알려줍니다. Linux에서는 추가 소프트웨어가 필요하지 않습니다.

**루프백 장치가 TUI에 표시되지 않습니다.**
→ macOS: 먼저 BlackHole을 설치하십시오(`brew install blackhole-2ch`). Windows/Linux: 루프백 장치는 기본 내장되어 있으며 자동으로 표시되어야 합니다.

**가상 장치에 샘플 레이트 대신 "adaptive"가 표시됩니다.**
→ 정상입니다. 가상 오디오 드라이버(BlackHole, VB-Cable)는 애플리케이션이 사용하는 샘플 레이트에 자동으로 적응하므로 표시된 레이트는 의미가 없습니다.

## CLI 모드

TUI에서 사용 가능한 모든 설정은 스크립팅과 자동화를 위한 CLI 플래그로도 전달할 수 있습니다:

```bash
# 서버
EchoWarp server -d 1 -P mypassword

# 클라이언트 (-a가 생략되면 LAN에서 서버를 자동 탐색)
EchoWarp client -a 192.168.1.10 -d 2 -P mypassword

# 오디오 장치 나열
EchoWarp devices
```

### 모드

각 모드에 대한 자세한 설명은 [스트리밍 모드](#스트리밍-모드)를 참조하세요.

| 모드 | 플래그 |
|------|--------|
| 일반 | *(기본값)* |
| 역방향 | `-r` |
| 듀플렉스 | `-X` |
| 컨퍼런스 | `--conference` |

### 공통 플래그

| 플래그 | 단축 | 설명 |
|--------|------|------|
| `--device` | `-d` | 오디오 장치 ID |
| `--device-name` | `-D` | 이름으로 장치 선택 (부분 문자열 일치) |
| `--password` | `-P` | 인증 비밀번호 |
| `--port` | `-p` | TCP 포트 (기본값: 4415) |
| `--sample-rate` | | 샘플 레이트 (기본값: 48000) |
| `--channels` | | 1=모노, 2=스테레오 (기본값: 1) |
| `--max-clients` | | 최대 연결 클라이언트 수 (기본값: 1) |
| `--virtual-mic` | | 가상 마이크 장치 생성 |
| `--config` | `-c` | YAML 설정 파일 로드 |
| `--save-config` | `-s` | 현재 설정을 YAML로 저장 |
| `--profile` | | 저장된 장치 프로필 로드 |
| `--stun-server` | | NAT 탐색을 위한 커스텀 STUN 서버 |
| `--tls-cert` / `--tls-key` | | TLS 인증서 및 키 |
| `--log-level` | | debug, info, warn, error |
| `--aec` | | 음향 에코 제거 (듀플렉스) |
| `--loopback` | | 시스템 오디오 캡처 (macOS, BlackHole 필요) |
| `--dry-run` | | 설정 유효성 검사 후 종료 |

### 설정 파일

설정 우선순위: CLI 플래그 > 환경 변수 (`ECHOWARP_*`) > 설정 파일 > 기본값.

TUI 설정 화면에서 설정 파일을 대화형으로 저장하고 로드할 수 있습니다. CLI 플래그도 사용할 수 있습니다:

```bash
# 현재 설정을 파일로 저장
EchoWarp server -d 1 -P pass --channels 2 -s myconfig.yml

# 파일에서 설정 로드
EchoWarp server -c myconfig.yml
```

## 설치

### 미리 빌드된 바이너리

[Releases](https://github.com/lHumaNl/EchoWarp/releases) 페이지에서 다운로드하세요. 다음 플랫폼을 지원합니다:
- **macOS** — arm64 (Apple Silicon), amd64 (Intel), `.app` 번들 포함
- **Linux** — amd64, arm64, armv7, armv6
- **Windows** — amd64, arm64

### 소스에서 빌드

Go 1.22+ 및 libopus 개발 헤더가 필요합니다.

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

결과 바이너리는 opus를 정적으로 링크합니다 — 런타임 의존성이 필요 없습니다.

## 시스템 요구 사항

- **macOS** 11.0+ (arm64, amd64)
- **Linux** (amd64, arm64, armv7, armv6)
- **Windows** 10+ (amd64, arm64)

가상 마이크의 경우:
- **macOS**: [BlackHole](https://github.com/ExistentialAudio/BlackHole) 오디오 드라이버
- **Windows**: [VB-Audio Virtual Cable](https://vb-audio.com/Cable/)
- **Linux**: PulseAudio (`pactl`)

## 네트워크 및 방화벽

### 서버 — 열어야 할 포트

| 포트 | 프로토콜 | 용도 |
|------|----------|------|
| `4415` | **TCP** | 시그널링 (인증 + WebRTC 핸드셰이크) |
| `4415` | **UDP** | 오디오 미디어 (WebRTC). 다중화 — 하나의 포트로 모든 클라이언트를 처리 |
| `4416` | **TCP** | 세션 정보 프로브 (클라이언트 자동 구성) |

> 포트 `4415`는 기본값이며 `--port`로 변경할 수 있습니다. 세션 정보 포트는 항상 `port + 1`입니다.

**요약:** 서버에서 인바운드로 **TCP 4415–4416** 및 **UDP 4415**를 열어주세요.

### 클라이언트 — 인바운드 포트 불필요

클라이언트는 아웃바운드 연결만 수행합니다. 클라이언트 측에서는 방화벽이나 포트 포워딩 규칙이 필요하지 않습니다.

### LAN 검색

mDNS 자동 검색(`Ctrl+F` 설정 화면에서)을 사용하는 경우 **UDP 멀티캐스트 포트 5353**을 허용하세요. `--no-discovery`로 비활성화할 수 있습니다.

### NAT 통과 (STUN / TURN)

EchoWarp는 STUN 서버를 사용하여 NAT를 통한 연결을 설정합니다. 양쪽 모두 대칭 NAT 뒤에 있어 직접 연결이 불가능한 경우, TURN 릴레이 서버를 구성할 수 있습니다 (설정 파일의 `turn_servers`). STUN과 TURN 모두 **아웃바운드** 연결이므로 인바운드 방화벽 규칙이 필요하지 않습니다.

## 라이선스

[MIT](../../LICENSE) — 서드파티 라이선스는 [THIRD_PARTY_NOTICES.md](../../THIRD_PARTY_NOTICES.md)를 참조하세요.
