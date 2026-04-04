<p align="center">
  <img src="../../build/icons/icon_512.png" alt="EchoWarp Logo" width="128">
</p>

<h1 align="center">EchoWarp</h1>

<p align="center">
  Echtzeit-Audioübertragung über das Netzwerk zwischen Hosts
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
  <a href="https://github.com/lHumaNl/EchoWarp/blob/main/LICENSE"><img src="https://img.shields.io/github/license/lHumaNl/EchoWarp?style=flat-square" alt="License"></a>
  <a href="https://github.com/lHumaNl/EchoWarp/releases"><img src="https://img.shields.io/github/downloads/lHumaNl/EchoWarp/total?style=flat-square" alt="Downloads"></a>
</p>

---

Audio auf einem Rechner aufnehmen und in Echtzeit über das Netzwerk auf einem anderen wiedergeben. EchoWarp nutzt WebRTC für den Transport und Opus für die Komprimierung und liefert latenzarmes Audio mit Ende-zu-Ende-Verschlüsselung.

## Funktionen

- **1:1-Streaming** — Server nimmt auf, Client gibt wieder (oder umgekehrt)
- **Duplex-Modus** — bidirektionales Audio, beide Seiten hören sich gegenseitig
- **Broadcast (1:N)** — ein Server überträgt an mehrere Clients
- **Konferenz (N:N)** — Mehrteilnehmer-Mixing mit persönlichen Mixes (Gesamt minus eigene Stimme)
- **Virtuelles Mikrofon** — empfangenes Audio erscheint als Mikrofon-Eingang (Discord, Zoom, OBS)
- **LAN-Erkennung** — automatische Servererkennung über mDNS
- **Aufnahme** — WAV-Aufnahme: gemischt, pro Spur oder beides
- **SIMD-Beschleunigung** — AVX/SSE (x86) und NEON (ARM) für Audio-Mixing
- **Ende-zu-Ende-Verschlüsselung** — AES + DTLS/SRTP
- **Plattformübergreifend** — macOS, Linux, Windows

## Schnellstart

1. Vorgefertigtes Binary von [Releases](https://github.com/lHumaNl/EchoWarp/releases) herunterladen (keine Abhängigkeiten — Opus ist statisch eingebunden)
2. `EchoWarp server` oder `EchoWarp client` ausführen
3. Alles im interaktiven TUI konfigurieren und Enter drücken, um zu starten

> **Windows-Benutzer:** Das Release-Archiv enthält `EchoWarp Server.bat` und `EchoWarp Client.bat` — einfach die gewünschte Datei doppelklicken. Es ist nicht nötig, cmd.exe manuell zu öffnen.

## Interaktives TUI

Das TUI ist die primäre Benutzeroberfläche von EchoWarp. Einfach `EchoWarp server` oder `EchoWarp client` starten — der interaktive Einrichtungsbildschirm öffnet sich automatisch.

### Einrichtungsbildschirm

Der Einrichtungsbildschirm hat ein zweispaltiges Layout:

**Linke Spalte — Audiogeräte.** Listet alle verfügbaren Ein-/Ausgabegeräte mit ihren Eigenschaften auf (Kanäle, Abtastrate, Bittiefe). Im Duplex- oder Konferenzmodus werden separate Aufnahme- `[C]`- und Wiedergabe- `[P]`-Rollen zugewiesen. Virtuelle Geräte können direkt aus der Liste heraus erstellt werden.

**Rechte Spalte — Einstellungen.** Alle Sitzungsparameter sind hier mit Inline-Validierung konfigurierbar:

| Einstellung | Beschreibung |
|-------------|--------------|
| Serveradresse | Server-IP/Hostname (nur Client) |
| Port | TCP-Port (Standard: 4415) |
| Passwort | Authentifizierungspasswort |
| Modus | Normal / Umgekehrt / Duplex |
| Max. Clients | Maximale Verbindungen (nur Server) |
| Abtastrate | 8000 / 16000 / 24000 / 48000 Hz |
| Kanäle | Mono / Stereo |
| Opus-Bitrate | 16–512 kbps |
| Echounterdrückung | AEC für Duplex-Modus |
| TLS | Aktivieren/Deaktivieren mit Zertifikat- und Schlüsselpfaden |
| Log-Level | debug / info / warn / error |

Zwischen den Spalten mit `Tab` wechseln, durch Felder mit den Pfeiltasten navigieren und `Enter` drücken, um zu bestätigen und das Streaming zu starten.

### LAN-Erkennung

Im Client-Einrichtungsbildschirm `Ctrl+F` drücken, um die Erkennungsüberlagerung zu öffnen. EchoWarp nutzt mDNS, um Server im lokalen Netzwerk automatisch zu finden — einen Server aus der Liste auswählen und die Adress-/Port-Felder werden automatisch ausgefüllt.

### Geräteprofile

`Ctrl+O` drücken, um die Profilüberlagerung zu öffnen. Aktuelle Gerätekonfiguration als benanntes Profil speichern, ein zuvor gespeichertes laden oder nicht mehr benötigte Profile löschen. Profile speichern Gerätezuweisungen und Rollen.

### Streaming-Bildschirm

Nach dem Verbinden zeigt das TUI den Echtzeit-Streaming-Status:

- **Verbindungsstatistiken** — RTT, Jitter, Paketverlust mit Schwellenbalken, die die Nähe zu Qualitätsgrenzen anzeigen
- **Verbindungsqualität** — Gesamtindikator (Ausgezeichnet / Gut / Befriedigend / Schlecht), berechnet aus RTT, Jitter und Verlust
- **Spektrumanalysator** — Echtzeit-FFT-Frequenzvisualisierung des Audiosignals
- **VU-Meter** — kanalweise Audiopegelmesser
- **Bitrate** — Live-Anzeige der Upload-/Download-Bitrate
- **Gerätesteuerung** — Lautstärkeanpassung, Stummschaltung, Gerätewechsel
- **Log-Panel** — ein-/ausblendbare scrollbare Protokollansicht

### TUI-Tastaturkürzel

| Taste | Aktion |
|-------|--------|
| `Tab` | Spalten wechseln (Einrichtung) |
| `Ctrl+F` | LAN-Erkennung (Client-Einrichtung) |
| `Ctrl+O` | Geräteprofile (Einrichtung) |
| `Enter` | Bestätigen und starten |
| `Ctrl+Q` | Beenden |
| `Ctrl+P` | Streaming pausieren/fortsetzen |
| `Ctrl+L` | Log-Panel ein-/ausblenden |
| `Ctrl+M` | Stummschalten/Stummschaltung aufheben |
| `+` / `-` | Lautstärke erhöhen/verringern |
| `Ctrl+Up/Down` | Logs scrollen |
| `Ctrl+R` | Aufnahme starten/stoppen |
| `Ctrl+D` | Client trennen (Server) |
| `Ctrl+B` | Client sperren (Server) |

## Streaming-Modi

EchoWarp verfügt über vier Streaming-Modi. Wählen Sie den passenden für Ihr Szenario — der Modus wird im TUI-Einrichtungsbildschirm (Serverseite) ausgewählt oder automatisch erkannt (Clientseite).

### Normal — Einweg: Server zu Clients

Der Server nimmt Audio auf und sendet es an alle verbundenen Clients. Die Clients hören nur zu.

```
Server [nimmt Audio auf] ───→ Client 1 [gibt Audio wieder]
                         ├──→ Client 2 [gibt Audio wieder]
                         └──→ Client N [gibt Audio wieder]
```

**Anwendungsfall:** Musik/Podcasts an Zuhörer streamen, Systemaudio (YouTube, Spotify) in einen anderen Raum übertragen, einen Loopback-Feed an einen entfernten Rechner senden.

**Benötigte Geräte:**

| Seite | Aufnahme (Eingang) | Wiedergabe (Ausgang) |
|-------|:-:|:-:|
| Server | erforderlich | — |
| Client | — | erforderlich |

**Teilnehmer:** 1 Server + 1…N Clients (im TUI unter *Max. Clients* einstellen).

---

### Umgekehrt — Einweg: Clients zu Server

Das Gegenteil von Normal. Die Clients nehmen Audio auf und senden es an den Server. Der Server hört nur zu.

```
Client 1 [nimmt Audio auf] ───→ Server [gibt Audio wieder]
Client 2 [nimmt Audio auf] ──┘
```

**Anwendungsfall:** Ein entferntes Mikrofon am Server-Rechner verwenden, Audio von einem entfernten Standort empfangen, das Mikrofon eines Clients an die Server-Lautsprecher senden.

**Benötigte Geräte:**

| Seite | Aufnahme (Eingang) | Wiedergabe (Ausgang) |
|-------|:-:|:-:|
| Server | — | erforderlich |
| Client | erforderlich | — |

**Teilnehmer:** 1 Server + 1…N Clients.

---

### Duplex — Bidirektional

Beide Seiten nehmen auf und geben gleichzeitig wieder. Alle hören alle.

```
Server [Mikrofon + Lautsprecher] ⟷ Client 1 [Mikrofon + Lautsprecher]
                                 ⟷ Client 2 [Mikrofon + Lautsprecher]
```

**Anwendungsfall:** Sprachanruf zwischen zwei oder mehr Rechnern, Gegensprechanlage zwischen Räumen, kollaborative Audio-Session.

**Benötigte Geräte:**

| Seite | Aufnahme (Eingang) | Wiedergabe (Ausgang) |
|-------|:-:|:-:|
| Server | erforderlich | erforderlich |
| Client | erforderlich | erforderlich |

**Teilnehmer:** 1 Server + 1…N Clients. Unterstützt AEC (Echounterdrückung) zur Vermeidung von Rückkopplungsschleifen.

---

### Konferenz — Mehrbenutzer-Mixing (N:N)

Der Server fungiert als Mixing-Hub. Jeder Teilnehmer (Server + Clients) sendet sein Audio und empfängt einen persönlichen Mix von **allen anderen** (Gesamt minus eigenes Signal). So hört man die eigene Stimme nicht.

```
Client 1 ──┐              ┌──→ Client 1 (hört Server + 2 + 3)
Client 2 ──┤  Server-Hub  ├──→ Client 2 (hört Server + 1 + 3)
Client 3 ──┘              └──→ Client 3 (hört Server + 1 + 2)
```

**Anwendungsfall:** Gruppenanrufe mit 3+ Rechnern, Remote-Proben, Konferenzen mit mehreren Teilnehmern.

**Benötigte Geräte:**

| Seite | Aufnahme (Eingang) | Wiedergabe (Ausgang) |
|-------|:-:|:-:|
| Server (nur Hub) | — | — |
| Server (Teilnehmer) | erforderlich | erforderlich |
| Client | erforderlich | erforderlich |

Der Server kann als **reiner Hub** (kein lokales Audio — nur Mixing für Clients) oder als vollwertiger Teilnehmer mit eigenem Mikrofon und Lautsprechern arbeiten.

**Teilnehmer:** 1 Server + 2…N Clients. Unterstützt Aufnahme (gemischt, pro Spur oder beides), Kick/Bann und Lautstärkeregelung pro Teilnehmer.

---

### Modusübersicht

| | Normal | Umgekehrt | Duplex | Konferenz |
|---|---|---|---|---|
| **Richtung** | Server → Clients | Clients → Server | Beide Richtungen | Alle ⟷ Alle |
| **Server-Geräte** | Nur Aufnahme | Nur Wiedergabe | Beides | Beides (oder keines als Hub) |
| **Client-Geräte** | Nur Wiedergabe | Nur Aufnahme | Beides | Beides |
| **Max. Teilnehmer** | 1 + N | 1 + N | 1 + N | 1 + N |
| **AEC-Unterstützung** | — | — | ja | ja |
| **Persönlicher Mix** | — | — | — | ja |
| **Aufnahme** | — | — | — | ja |
| **CLI-Flag** | *(Standard)* | `-r` | `-X` | `--conference` |

> **Tipp:** Die Anzahl der Clients wird über die Einstellung *Max. Clients* im TUI gesteuert (Standard: 1). Für Broadcast- oder Mehrbenutzer-Szenarien höher einstellen.

## Audio-Routing-Leitfaden

### Anwendungsfälle

EchoWarp unterstützt verschiedene Audio-Routing-Szenarien. Die nachfolgende Tabelle zeigt, welcher Modus und welche Gerätekonfiguration für die jeweiligen Fälle verwendet werden:

| Szenario | Modus | Server-Geräte | Client-Geräte |
|----------|-------|---------------|---------------|
| Mikrofon zu entfernten Lautsprechern streamen | Normal | Mikrofon `[Input]` | Lautsprecher `[Output]` |
| Systemaudio streamen (YouTube, Spotify) | Normal | 🔄 Loopback `[Input]` | Lautsprecher `[Output]` |
| Entferntes Mikrofon in Discord/Zoom verwenden | Reverse | Lautsprecher `[Output]` | Mikrofon `[Input]` |
| Bidirektionaler Sprach-Chat | Duplex | Mikrofon + Lautsprecher | Mikrofon + Lautsprecher |
| Gruppenanruf (3+ Geräte) | Conference | — (Hub) | Mikrofon + Lautsprecher |
| Entferntes Audio als virtuelles Mikrofon in OBS | Normal | 🔄 Loopback `[Input]` | ⟡ Virtuelles Mikrofon `[Output]` |
| Stream + lokales Mikrofon als ein virtuelles Mikrofon | Normal | Mikrofon `[Input]` | ⟡ Virtuell `[Output]` + 🎤 Mix-Mikrofon |

### Loopback — Systemaudio aufnehmen

Loopback-Geräte nehmen das gesamte auf einem Gerät abgespielte Audio auf (Musik, Videoanrufe, Spielsound) und stellen es als Eingangsquelle bereit.

| Plattform | Funktionsweise | Einrichtung |
|-----------|---------------|-------------|
| **macOS** | Aggregiertes Gerät (Lautsprecher + BlackHole) | [BlackHole](https://github.com/ExistentialAudio/BlackHole) installieren: `brew install blackhole-2ch`. Loopback-Geräte erscheinen automatisch im TUI |
| **Windows** | WASAPI nativer Loopback | Integriert, keine zusätzliche Software erforderlich |
| **Linux** | PulseAudio Monitor-Quellen | Integriert mit PulseAudio/PipeWire |

Im TUI sind Loopback-Geräte mit 🔄 gekennzeichnet und erscheinen im Abschnitt Input.

### Virtuelles Mikrofon — Audio an andere Apps weiterleiten

Ein virtuelles Mikrofon lässt empfangenes Audio als Mikrofon-Eingang erscheinen, den Discord, Zoom, OBS und andere Anwendungen nutzen können.

| Plattform | Treiber | Installation |
|-----------|---------|--------------|
| **macOS** | [BlackHole](https://github.com/ExistentialAudio/BlackHole) | `brew install blackhole-2ch` |
| **Windows** | [VB-Audio Virtual Cable](https://vb-audio.com/Cable/) | Download von vb-audio.com |
| **Linux** | PulseAudio Null-Sink | Im TUI erstellt: Einstellungen → Virtuelles Mikrofon → Erstellen |

Unter macOS und Windows den Treiber installieren und im Abschnitt Output des TUI auswählen. Unter Linux erstellt das TUI den virtuellen Sink automatisch — in anderen Apps **„Monitor of EchoWarp"** als Mikrofon auswählen.

Virtuelle Geräte sind im TUI mit ⟡ gekennzeichnet und zeigen **adaptive** statt einer festen Abtastrate an.

### Lokales Mikrofon in die virtuelle Ausgabe mischen

Wenn Sie einen Remote-Stream an ein virtuelles Ausgabegerät (BlackHole, VB-Cable) weiterleiten, sehen Drittanbieter-Apps wie Discord oder Zoom nur den Stream — Ihr lokales Mikrofon wird nicht gehört. Wenn sowohl Ihre Stimme **als auch** der Stream als ein einziger Mikrofon-Eingang erscheinen sollen, kann EchoWarp beides zusammenmischen.

**Anwendung:** Im TUI ein virtuelles Gerät im Abschnitt Output auswählen. Darunter erscheinen Eingabegeräte als Unterpunkte — das gewünschte Mikrofon zum Mischen aktivieren:

```
🔊 Output
  ⟡ BlackHole 2ch                           stereo 32-bit adaptive   [✓]
      └─ 🎤 Built-in Microphone                                      [✓]
```

Das gemischte Audio (Stream + Mikrofon) wird in die virtuelle Ausgabe geschrieben. In Discord/Zoom/OBS dieses virtuelle Gerät als Mikrofon auswählen — sowohl der Stream als auch Ihre Stimme werden gehört.

**Alternativen auf Betriebssystemebene** (ohne den integrierten Mixer):

| Plattform | Methode | Details |
|-----------|---------|---------|
| **macOS** | Aggregiertes Gerät | Audio-MIDI-Setup öffnen → Aggregiertes Gerät erstellen, das Mikrofon + BlackHole kombiniert. Das Aggregat als Eingang in der App auswählen |
| **Windows** | [VoiceMeeter](https://vb-audio.com/Voicemeeter/) | Kostenloser virtueller Mixer — VB-Cable und Mikrofon in VoiceMeeter routen, dessen Ausgang als Mikrofon in der App verwenden |
| **Linux** | PulseAudio combine-source | `pactl load-module module-combine-source source_name=combined slaves=<mic>,<virtual>.monitor` |

Der integrierte Mixer funktioniert auf allen Plattformen und erfordert keine zusätzliche Konfiguration.

### Geräteabschnitte

Das TUI zeigt nur die für den aktuellen Modus relevanten Abschnitte:

| Modus + Seite | Sichtbare Abschnitte |
|---------------|---------------------|
| Normal-Server / Reverse-Client | 🎤 Nur Input |
| Normal-Client / Reverse-Server | 🔊 Nur Output |
| Duplex / Conference | 🎤 Input + 🔊 Output |

### Diagnose

`EchoWarp doctor` ausführen, um die Audio-Konfiguration zu überprüfen:

```
Audio
[OK] Audio system: 2 input device(s), 2 output device(s)
[OK] Virtual audio: BlackHole 2ch detected
```

Wenn kein virtueller Audiotreiber gefunden wird, zeigt der Doctor Installationsanweisungen für die jeweilige Plattform an.

### FAQ

**Ich möchte Musik von meinem Computer auf Lautsprechern in einem anderen Raum hören.**
→ `EchoWarp server` auf dem Musik-Gerät ausführen, das 🔄 Loopback-Gerät (die eigenen Lautsprecher) unter Input auswählen. `EchoWarp client` auf dem entfernten Gerät ausführen, Lautsprecher unter Output auswählen.

**Ich möchte ein entferntes Mikrofon für Zoom/Discord verwenden.**
→ `EchoWarp server` im Reverse-Modus auf dem entfernten Gerät (mit dem Mikrofon) ausführen. `EchoWarp client` auf dem eigenen Gerät ausführen, ein virtuelles Audiogerät (BlackHole/VB-Cable) unter Output auswählen. In Zoom/Discord dieses virtuelle Gerät als Mikrofon auswählen.

**Ich möchte Systemaudio (YouTube, Spotify) auf ein anderes Gerät streamen.**
→ Gleich wie „Musik in einem anderen Raum" — 🔄 Loopback auf der Server-Seite verwenden. Das Loopback-Gerät nimmt alles auf, was über die ausgewählten Lautsprecher abgespielt wird.

**Ich möchte, dass entferntes Audio in OBS als Mikrofon-Eingang erscheint.**
→ Server mit der Audioquelle ausführen. Auf dem eigenen Gerät (Client) ein virtuelles Audiogerät unter Output auswählen. In OBS eine Quelle „Audio-Eingangserfassung" hinzufügen und das virtuelle Gerät auswählen.

**Ich möchte einen bidirektionalen Sprach-Chat zwischen zwei Computern.**
→ Duplex-Modus verwenden. Auf beiden Geräten ein Mikrofon unter Input und Lautsprecher unter Output auswählen.

**Ich möchte einen Gruppenanruf mit 3+ Geräten.**
→ Conference-Modus verwenden. Der Server fungiert als Hub (keine Geräte erforderlich). Jeder Client wählt ein Mikrofon und Lautsprecher. Alle hören alle anderen, abzüglich der eigenen Stimme.

**Ich möchte, dass Discord sowohl den Remote-Stream ALS AUCH meine Stimme über ein virtuelles Mikrofon hört.**
→ Auf dem Client ein virtuelles Gerät (BlackHole/VB-Cable) unter Output auswählen. Darunter erscheint eine Liste der Eingabegeräte — das eigene Mikrofon aktivieren. EchoWarp mischt den Stream und das Mikrofon in die virtuelle Ausgabe. In Discord das virtuelle Gerät als Mikrofon auswählen.

**Ich habe BlackHole / VB-Cable nicht installiert.**
→ `EchoWarp doctor` ausführen — es zeigt genau an, was installiert werden muss und wie. Unter Linux ist keine zusätzliche Software erforderlich.

**Das Loopback-Gerät erscheint nicht im TUI.**
→ macOS: Zuerst BlackHole installieren (`brew install blackhole-2ch`). Windows/Linux: Loopback-Geräte sind integriert und sollten automatisch erscheinen.

**Das virtuelle Gerät zeigt „adaptive" statt einer Abtastrate an.**
→ Das ist normal. Virtuelle Audiotreiber (BlackHole, VB-Cable) passen sich an die Abtastrate der verwendenden Anwendung an — die angezeigte Rate ist nicht aussagekräftig.

## CLI-Modus

Alle im TUI verfügbaren Einstellungen können auch als CLI-Flags für Skripting und Automatisierung übergeben werden:

```bash
# Server
EchoWarp server -d 1 -P mypassword

# Client (erkennt Server im LAN automatisch, wenn -a weggelassen wird)
EchoWarp client -a 192.168.1.10 -d 2 -P mypassword

# Audiogeräte auflisten
EchoWarp devices
```

### Modi

Siehe [Streaming-Modi](#streaming-modi) für detaillierte Beschreibungen der einzelnen Modi.

| Modus | Flag |
|-------|------|
| Normal | *(Standard)* |
| Umgekehrt | `-r` |
| Duplex | `-X` |
| Konferenz | `--conference` |

### Allgemeine Flags

| Flag | Kurz | Beschreibung |
|------|------|--------------|
| `--device` | `-d` | Audiogeräte-ID |
| `--device-name` | `-D` | Gerät nach Name auswählen (Teilstring-Suche) |
| `--password` | `-P` | Authentifizierungspasswort |
| `--port` | `-p` | TCP-Port (Standard: 4415) |
| `--sample-rate` | | Abtastrate (Standard: 48000) |
| `--channels` | | 1=Mono, 2=Stereo (Standard: 1) |
| `--max-clients` | | Maximale verbundene Clients (Standard: 1) |
| `--virtual-mic` | | Virtuelles Mikrofon-Gerät erstellen |
| `--config` | `-c` | YAML-Konfigurationsdatei laden |
| `--save-config` | `-s` | Aktuelle Einstellungen als YAML speichern |
| `--profile` | | Gespeichertes Geräteprofil laden |
| `--stun-server` | | Benutzerdefinierte STUN-Server für NAT-Traversal |
| `--tls-cert` / `--tls-key` | | TLS-Zertifikat und -Schlüssel |
| `--log-level` | | debug, info, warn, error |
| `--aec` | | Akustische Echounterdrückung (Duplex) |
| `--loopback` | | Systemaudio aufnehmen (macOS, erfordert BlackHole) |
| `--dry-run` | | Konfiguration validieren und beenden |

### Konfigurationsdateien

Einstellungspriorität: CLI-Flags > Umgebungsvariablen (`ECHOWARP_*`) > Konfigurationsdatei > Standardwerte.

Der TUI-Einrichtungsbildschirm ermöglicht das interaktive Speichern und Laden von Konfigurationsdateien. Sie können auch CLI-Flags verwenden:

```bash
# Aktuelle Einstellungen in eine Datei speichern
EchoWarp server -d 1 -P pass --channels 2 -s myconfig.yml

# Einstellungen aus einer Datei laden
EchoWarp server -c myconfig.yml
```

## Installation

### Vorgefertigte Binaries

Von der [Releases](https://github.com/lHumaNl/EchoWarp/releases)-Seite herunterladen. Verfügbar für:
- **macOS** — arm64 (Apple Silicon), amd64 (Intel), inkl. `.app`-Bundle
- **Linux** — amd64, arm64, armv7, armv6
- **Windows** — amd64, arm64

### Aus dem Quellcode bauen

Erfordert Go 1.22+ und libopus-Entwicklungs-Header.

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

Das resultierende Binary verknüpft Opus statisch — keine Laufzeitabhängigkeit erforderlich.

## Systemanforderungen

- **macOS** 11.0+ (arm64, amd64)
- **Linux** (amd64, arm64, armv7, armv6)
- **Windows** 10+ (amd64, arm64)

Für virtuelles Mikrofon:
- **macOS**: [BlackHole](https://github.com/ExistentialAudio/BlackHole)-Audiotreiber
- **Linux**: PulseAudio (`pactl`)

## Netzwerk & Firewall

### Server — zu öffnende Ports

| Port | Protokoll | Zweck |
|------|-----------|-------|
| `4415` | **TCP** | Signalisierung (Authentifizierung + WebRTC-Handshake) |
| `4415` | **UDP** | Audio-Medien (WebRTC). Multiplexed — ein Port bedient alle Clients |
| `4416` | **TCP** | Sitzungsinfo-Abfrage (automatische Client-Konfiguration) |

> Port `4415` ist der Standardwert und kann mit `--port` geändert werden. Der Sitzungsinfo-Port ist immer `port + 1`.

**Kurzfassung:** Öffnen Sie **TCP 4415–4416** und **UDP 4415** eingehend auf dem Server.

### Client — keine eingehenden Ports erforderlich

Der Client stellt nur ausgehende Verbindungen her. Auf der Client-Seite sind keine Firewall- oder Portweiterleitungsregeln erforderlich.

### LAN-Erkennung

Wenn Sie die mDNS-Autoerkennung verwenden (`Ctrl+F` im Setup), erlauben Sie **UDP-Multicast auf Port 5353**. Dies kann mit `--no-discovery` deaktiviert werden.

### NAT-Traversal (STUN / TURN)

EchoWarp verwendet STUN-Server, um Verbindungen durch NAT herzustellen. Wenn sich beide Seiten hinter symmetrischem NAT befinden und keine direkte Verbindung aufgebaut werden kann, lässt sich ein TURN-Relay-Server konfigurieren (`turn_servers` in der Konfiguration). Sowohl STUN als auch TURN sind **ausgehende** Verbindungen und erfordern keine eingehenden Firewall-Regeln.

## Lizenz

[MIT](../../LICENSE) — siehe [THIRD_PARTY_NOTICES.md](../../THIRD_PARTY_NOTICES.md) für Drittanbieter-Lizenzen.
