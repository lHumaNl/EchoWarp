<p align="center">
  <img src="../../build/icons/icon_512.png" alt="EchoWarp Logo" width="128">
</p>

<h1 align="center">EchoWarp</h1>

<p align="center">
  Strumieniowanie audio w czasie rzeczywistym między hostami przez sieć
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

Przechwytuj dźwięk na jednej maszynie i odtwarzaj go na drugiej — w czasie rzeczywistym przez sieć. EchoWarp używa WebRTC do transportu i Opus do kompresji, zapewniając audio o niskich opóźnieniach z szyfrowaniem end-to-end.

## Funkcje

- **Strumieniowanie 1:1** — serwer przechwytuje, klient odtwarza (lub odwrotnie)
- **Tryb dupleksowy** — dwukierunkowe audio, obie strony słyszą się nawzajem
- **Broadcast (1:N)** — jeden serwer strumieniuje do wielu klientów
- **Konferencja (N:N)** — miksowanie dla wielu uczestników z osobistymi miksami (suma minus własny sygnał)
- **Wirtualny mikrofon** — odebrane audio pojawia się jako wejście mikrofonu (Discord, Zoom, OBS)
- **Odkrywanie w sieci LAN** — automatyczne wykrywanie serwerów przez mDNS
- **Nagrywanie** — zapis WAV: zmiksowany, na osobnych ścieżkach lub oba
- **Akceleracja SIMD** — AVX/SSE (x86) i NEON (ARM) do miksowania audio
- **Szyfrowanie end-to-end** — AES + DTLS/SRTP
- **Wieloplatformowość** — macOS, Linux, Windows

## Szybki start

1. Pobierz gotowy plik binarny ze strony [Releases](https://github.com/lHumaNl/EchoWarp/releases) (brak zależności — opus jest statycznie zlinkowany)
2. Uruchom `EchoWarp server` lub `EchoWarp client`
3. Skonfiguruj wszystko w interaktywnym interfejsie TUI i naciśnij Enter, aby rozpocząć

> **Użytkownicy Windows:** Archiwum z wydaniem zawiera pliki `EchoWarp Server.bat` i `EchoWarp Client.bat` — wystarczy dwukrotnie kliknąć odpowiedni plik. Nie trzeba ręcznie otwierać cmd.exe.

## Interaktywny interfejs TUI

TUI jest głównym interfejsem EchoWarp. Wystarczy uruchomić `EchoWarp server` lub `EchoWarp client` — ekran interaktywnej konfiguracji otwiera się automatycznie.

### Ekran konfiguracji

Ekran konfiguracji ma układ dwukolumnowy:

**Lewa kolumna — Urządzenia audio.** Wyświetla wszystkie dostępne urządzenia wejściowe/wyjściowe wraz z ich właściwościami (kanały, częstotliwość próbkowania, głębia bitowa). W trybie dupleksowym lub konferencyjnym można przypisywać osobne role: przechwytywanie `[C]` i odtwarzanie `[P]`. Tworzenie urządzeń wirtualnych jest dostępne bezpośrednio z listy.

**Prawa kolumna — Ustawienia.** Wszystkie parametry sesji można tu konfigurować z walidacją w czasie rzeczywistym:

| Ustawienie | Opis |
|---------|-------------|
| Adres serwera | IP/nazwa hosta serwera (tylko klient) |
| Port | Port TCP (domyślnie: 4415) |
| Hasło | Hasło uwierzytelniające |
| Tryb | Normalny / Odwrotny / Dupleks |
| Maks. klientów | Maksymalna liczba połączeń (tylko serwer) |
| Częstotliwość próbkowania | 8000 / 16000 / 24000 / 48000 Hz |
| Kanały | Mono / Stereo |
| Bitrate Opus | 16–512 kbps |
| Tłumienie echa | AEC dla trybu dupleksowego |
| TLS | Włącz/wyłącz ze ścieżkami certyfikatu i klucza |
| Poziom logowania | debug / info / warn / error |

Przełączaj się między kolumnami klawiszem `Tab`, poruszaj się po polach strzałkami, a następnie naciśnij `Enter`, aby potwierdzić i rozpocząć strumieniowanie.

### Odkrywanie w sieci LAN

Na ekranie konfiguracji klienta naciśnij `Ctrl+F`, aby otworzyć nakładkę odkrywania. EchoWarp używa mDNS do automatycznego wykrywania serwerów w sieci lokalnej — wybierz jeden z listy, a pola adresu i portu zostaną wypełnione automatycznie.

### Profile urządzeń

Naciśnij `Ctrl+O`, aby otworzyć nakładkę profili. Zapisz bieżącą konfigurację urządzeń jako nazwany profil, wczytaj wcześniej zapisany lub usuń profile, których już nie potrzebujesz. Profile zapamiętują przypisania urządzeń i ich role.

### Ekran strumieniowania

Po nawiązaniu połączenia TUI wyświetla stan strumieniowania w czasie rzeczywistym:

- **Statystyki połączenia** — RTT, jitter, utrata pakietów z paskami progowymi pokazującymi zbliżanie się do limitów jakości
- **Jakość połączenia** — ogólny wskaźnik (Doskonała / Dobra / Dostateczna / Słaba) obliczany na podstawie RTT, jittera i utraty pakietów
- **Analizator widma** — wizualizacja FFT częstotliwości sygnału audio w czasie rzeczywistym
- **Wskaźniki VU** — mierniki poziomu audio dla każdego kanału
- **Bitrate** — aktualne wyświetlanie bitrate przesyłania i pobierania
- **Sterowanie urządzeniami** — regulacja głośności, wyciszenie, przełączanie urządzeń
- **Panel dziennika** — przewijany widok dziennika z możliwością przełączania

### Skróty klawiszowe TUI

| Klawisz | Akcja |
|-----|--------|
| `Tab` | Przełącz kolumny (konfiguracja) |
| `Ctrl+F` | Odkrywanie LAN (konfiguracja klienta) |
| `Ctrl+O` | Profile urządzeń (konfiguracja) |
| `Enter` | Potwierdź i uruchom |
| `Ctrl+Q` | Wyjdź |
| `Ctrl+P` | Wstrzymaj/wznów strumieniowanie |
| `Ctrl+L` | Pokaż/ukryj panel dziennika |
| `Ctrl+M` | Wycisz/odwycisz |
| `+` / `-` | Zwiększ/zmniejsz głośność |
| `Ctrl+Up/Down` | Przewijaj dzienniki |
| `Ctrl+R` | Rozpocznij/zatrzymaj nagrywanie |
| `Ctrl+D` | Rozłącz klienta (serwer) |
| `Ctrl+B` | Zablokuj klienta (serwer) |

## Tryby strumieniowania

EchoWarp ma cztery tryby strumieniowania. Wybierz ten, który pasuje do Twojego scenariusza — tryb ustawia się na ekranie konfiguracji TUI (po stronie serwera) lub jest wykrywany automatycznie (po stronie klienta).

### Normalny — jednokierunkowy: serwer do klientów

Serwer przechwytuje dźwięk i wysyła go do wszystkich podłączonych klientów. Klienci tylko słuchają.

```
Server [captures audio] ───→ Client 1 [plays audio]
                         ├──→ Client 2 [plays audio]
                         └──→ Client N [plays audio]
```

**Kiedy używać:** strumieniowanie muzyki/podcastów do słuchaczy, rozgłaszanie dźwięku systemowego (YouTube, Spotify) do innego pokoju, wysyłanie sygnału loopback do zdalnej maszyny.

**Wymagane urządzenia:**

| Strona | Przechwytywanie (wejście) | Odtwarzanie (wyjście) |
|------|:-:|:-:|
| Serwer | wymagane | — |
| Klient | — | wymagane |

**Uczestnicy:** 1 serwer + 1…N klientów (ustaw *Maks. klientów* w TUI).

---

### Odwrotny — jednokierunkowy: klienci do serwera

Odwrotność trybu Normalnego. Klienci przechwytują dźwięk i wysyłają go do serwera. Serwer tylko słucha.

```
Client 1 [captures audio] ───→ Server [plays audio]
Client 2 [captures audio] ──┘
```

**Kiedy używać:** użycie zdalnego mikrofonu na maszynie serwera, zbieranie dźwięku ze zdalnej lokalizacji, wysyłanie mikrofonu klienta na głośniki serwera.

**Wymagane urządzenia:**

| Strona | Przechwytywanie (wejście) | Odtwarzanie (wyjście) |
|------|:-:|:-:|
| Serwer | — | wymagane |
| Klient | wymagane | — |

**Uczestnicy:** 1 serwer + 1…N klientów.

---

### Dupleks — dwukierunkowy

Obie strony przechwytują i odtwarzają jednocześnie. Wszyscy słyszą wszystkich.

```
Server [mic + speakers] ⟷ Client 1 [mic + speakers]
                        ⟷ Client 2 [mic + speakers]
```

**Kiedy używać:** rozmowa głosowa między dwoma lub więcej maszynami, interkom między pokojami, wspólna sesja audio.

**Wymagane urządzenia:**

| Strona | Przechwytywanie (wejście) | Odtwarzanie (wyjście) |
|------|:-:|:-:|
| Serwer | wymagane | wymagane |
| Klient | wymagane | wymagane |

**Uczestnicy:** 1 serwer + 1…N klientów. Obsługuje AEC (tłumienie echa) w celu zapobiegania sprzężeniom zwrotnym.

---

### Konferencja — miksowanie wieloużytkownikowe (N:N)

Serwer działa jako węzeł miksujący. Każdy uczestnik (serwer + klienci) wysyła swój dźwięk i otrzymuje osobisty miks **wszystkich pozostałych** (suma minus własny sygnał). Dzięki temu nie słyszysz własnego głosu.

```
Client 1 ──┐              ┌──→ Client 1 (hears Server + 2 + 3)
Client 2 ──┤  Server hub  ├──→ Client 2 (hears Server + 1 + 3)
Client 3 ──┘              └──→ Client 3 (hears Server + 1 + 2)
```

**Kiedy używać:** rozmowy grupowe z 3+ maszynami, zdalne próby, konferencje wielouczestnikowe.

**Wymagane urządzenia:**

| Strona | Przechwytywanie (wejście) | Odtwarzanie (wyjście) |
|------|:-:|:-:|
| Serwer (tylko hub) | — | — |
| Serwer (uczestnik) | wymagane | wymagane |
| Klient | wymagane | wymagane |

Serwer może pracować jako **tylko hub** (bez lokalnego audio — jedynie miksowanie dla klientów) lub jako pełny uczestnik z własnym mikrofonem i głośnikami.

**Uczestnicy:** 1 serwer + 2…N klientów. Obsługuje nagrywanie (zmiksowane, na osobnych ścieżkach lub oba), rozłączanie/blokowanie oraz regulację głośności dla każdego uczestnika.

---

### Podsumowanie trybów

| | Normalny | Odwrotny | Dupleks | Konferencja |
|---|---|---|---|---|
| **Kierunek** | Serwer → Klienci | Klienci → Serwer | Obie strony | Wszyscy ⟷ Wszyscy |
| **Urządzenia serwera** | Tylko przechwytywanie | Tylko odtwarzanie | Oba | Oba (lub brak jeśli hub) |
| **Urządzenia klienta** | Tylko odtwarzanie | Tylko przechwytywanie | Oba | Oba |
| **Maks. uczestników** | 1 + N | 1 + N | 1 + N | 1 + N |
| **Obsługa AEC** | — | — | tak | tak |
| **Osobisty miks** | — | — | — | tak |
| **Nagrywanie** | — | — | — | tak |
| **Flaga CLI** | *(domyślnie)* | `-r` | `-X` | `--conference` |

> **Wskazówka:** Liczbę klientów kontroluje ustawienie *Maks. klientów* w TUI (domyślnie: 1). Ustaw wyższą wartość, jeśli potrzebujesz rozgłaszania lub scenariuszy wieloużytkownikowych.

## Przewodnik po routingu audio

### Przypadki użycia

EchoWarp obsługuje kilka scenariuszy routingu audio. Poniższa tabela pokazuje, który tryb i konfigurację urządzeń zastosować w każdym przypadku:

| Scenariusz | Tryb | Urządzenia serwera | Urządzenia klienta |
|------------|------|--------------------|--------------------|
| Przesyłaj mikrofon do zdalnych głośników | Normal | Mikrofon `[Input]` | Głośniki `[Output]` |
| Przesyłaj dźwięk systemowy (YouTube, Spotify) | Normal | 🔄 Loopback `[Input]` | Głośniki `[Output]` |
| Użyj zdalnego mikrofonu w Discord/Zoom | Reverse | Głośniki `[Output]` | Mikrofon `[Input]` |
| Dwukierunkowy czat głosowy | Duplex | Mikrofon + Głośniki | Mikrofon + Głośniki |
| Rozmowa grupowa (3+ maszyny) | Conference | — (hub) | Mikrofon + Głośniki |
| Zdalny dźwięk jako wirtualny mikrofon w OBS | Normal | 🔄 Loopback `[Input]` | ⟡ Wirtualny mikrofon `[Output]` |
| Strumień + lokalny mikrofon jako jeden wirtualny mikrofon | Normal | Mikrofon `[Input]` | ⟡ Wirtualny `[Output]` + 🎤 Miks mikrofonu |

### Loopback — przechwytywanie dźwięku systemowego

Urządzenia loopback przechwytują cały dźwięk odtwarzany na maszynie (muzyka, rozmowy wideo, dźwięk gier) i udostępniają go jako źródło wejściowe.

| Platforma | Jak działa | Konfiguracja |
|-----------|------------|--------------|
| **macOS** | Aggregate Device (głośniki + BlackHole) | Zainstaluj [BlackHole](https://github.com/ExistentialAudio/BlackHole): `brew install blackhole-2ch`. Urządzenia loopback pojawiają się w TUI automatycznie |
| **Windows** | Natywny loopback WASAPI | Wbudowany, nie wymaga dodatkowego oprogramowania |
| **Linux** | Źródła monitorujące PulseAudio | Wbudowany w PulseAudio/PipeWire |

W TUI urządzenia loopback są oznaczone symbolem 🔄 i pojawiają się w sekcji Input.

### Wirtualny mikrofon — kierowanie dźwięku do innych aplikacji

Wirtualny mikrofon sprawia, że odbierany dźwięk pojawia się jako wejście mikrofonowe, z którego mogą korzystać Discord, Zoom, OBS i inne aplikacje.

| Platforma | Sterownik | Instalacja |
|-----------|-----------|------------|
| **macOS** | [BlackHole](https://github.com/ExistentialAudio/BlackHole) | `brew install blackhole-2ch` |
| **Windows** | [VB-Audio Virtual Cable](https://vb-audio.com/Cable/) | Pobierz z vb-audio.com |
| **Linux** | PulseAudio null-sink | Tworzony z TUI: Ustawienia → Wirtualny mikrofon → Utwórz |

Na macOS i Windows zainstaluj sterownik i wybierz go w sekcji Output w TUI. Na Linuksie TUI tworzy wirtualny sink automatycznie — w innych aplikacjach wybierz **„Monitor of EchoWarp"** jako mikrofon.

Urządzenia wirtualne są oznaczone symbolem ⟡ w TUI i wyświetlają **adaptive** zamiast stałej częstotliwości próbkowania.

### Miksowanie lokalnego mikrofonu do wyjścia wirtualnego

Gdy kierujesz zdalny strumień do wirtualnego urządzenia wyjściowego (BlackHole, VB-Cable), aplikacje takie jak Discord czy Zoom widzą tylko strumień — nie słyszą Twojego lokalnego mikrofonu. Jeśli potrzebujesz, aby zarówno Twój głos, **jak i** strumień pojawiały się jako jedno wejście mikrofonowe, EchoWarp może je zmiksować razem.

**Jak używać:** W TUI wybierz wirtualne urządzenie w sekcji Output. Urządzenia wejściowe pojawią się poniżej jako podelementy — zaznacz mikrofon, który chcesz domiksować:

```
🔊 Output
  ⟡ BlackHole 2ch                           stereo 32-bit adaptive   [✓]
      └─ 🎤 Built-in Microphone                                      [✓]
```

Zmiksowane audio (strumień + mikrofon) jest zapisywane na wyjście wirtualne. W Discord/Zoom/OBS wybierz to urządzenie wirtualne jako mikrofon — będzie słychać zarówno strumień, jak i Twój głos.

**Alternatywy na poziomie systemu operacyjnego** (bez użycia wbudowanego miksera):

| Platforma | Sposób | Szczegóły |
|-----------|--------|-----------|
| **macOS** | Aggregate Device | Otwórz Audio MIDI Setup → Utwórz Aggregate Device łączące mikrofon + BlackHole. Wybierz agregat jako wejście w aplikacji |
| **Windows** | [VoiceMeeter](https://vb-audio.com/Voicemeeter/) | Darmowy wirtualny mikser — podłącz zarówno VB-Cable, jak i mikrofon do VoiceMeeter, użyj jego wyjścia jako mikrofonu w aplikacji |
| **Linux** | PulseAudio combine-source | `pactl load-module module-combine-source source_name=combined slaves=<mic>,<virtual>.monitor` |

Wbudowany mikser działa na wszystkich platformach i nie wymaga dodatkowej konfiguracji.

### Sekcje urządzeń

TUI wyświetla tylko sekcje istotne dla bieżącego trybu:

| Tryb + Strona | Widoczne sekcje |
|---------------|-----------------|
| Serwer Normal / Klient Reverse | tylko 🎤 Input |
| Klient Normal / Serwer Reverse | tylko 🔊 Output |
| Duplex / Conference | 🎤 Input + 🔊 Output |

### Diagnostyka

Uruchom `EchoWarp doctor`, aby sprawdzić konfigurację audio:

```
Audio
[OK] Audio system: 2 input device(s), 2 output device(s)
[OK] Virtual audio: BlackHole 2ch detected
```

Jeśli żaden wirtualny sterownik audio nie zostanie znaleziony, doctor wyświetli instrukcje instalacji dla Twojej platformy.

### FAQ

**Chcę słuchać muzyki z komputera na głośnikach w innym pokoju.**
→ Uruchom `EchoWarp server` na maszynie z muzyką, wybierz urządzenie 🔄 loopback (Twoje głośniki) w Input. Uruchom `EchoWarp client` na zdalnej maszynie, wybierz głośniki w Output.

**Chcę używać zdalnego mikrofonu w Zoom/Discord.**
→ Uruchom `EchoWarp server` w trybie Reverse na zdalnej maszynie (tej z mikrofonem). Uruchom `EchoWarp client` na swojej maszynie, wybierz wirtualne urządzenie audio (BlackHole/VB-Cable) w Output. W Zoom/Discord wybierz to wirtualne urządzenie jako mikrofon.

**Chcę przesyłać dźwięk systemowy (YouTube, Spotify) na inną maszynę.**
→ Tak samo jak „muzyka w innym pokoju" — użyj 🔄 loopback po stronie serwera. Urządzenie loopback przechwytuje wszystko odtwarzane przez wybrane głośniki.

**Chcę, żeby zdalny dźwięk pojawiał się jako wejście mikrofonowe w OBS.**
→ Uruchom serwer ze źródłem dźwięku. Na swojej maszynie (klient) wybierz wirtualne urządzenie audio w Output. W OBS dodaj źródło „Audio Input Capture" i wybierz urządzenie wirtualne.

**Chcę prowadzić dwukierunkowy czat głosowy między dwoma komputerami.**
→ Użyj trybu Duplex. Na obu maszynach wybierz mikrofon w Input i głośniki w Output.

**Chcę prowadzić rozmowę grupową z 3+ maszynami.**
→ Użyj trybu Conference. Serwer działa jako hub (nie są potrzebne urządzenia). Każdy klient wybiera mikrofon i głośniki. Wszyscy słyszą wszystkich, z wyjątkiem własnego głosu.

**Chcę, żeby Discord słyszał zarówno zdalny strumień, JAK I mój głos przez jeden wirtualny mikrofon.**
→ Na kliencie wybierz wirtualne urządzenie (BlackHole/VB-Cable) w Output. Poniżej pojawi się lista urządzeń wejściowych — zaznacz swój mikrofon. EchoWarp zmikuje strumień i Twój mikrofon do wyjścia wirtualnego. W Discord wybierz urządzenie wirtualne jako mikrofon.

**Nie mam zainstalowanego BlackHole / VB-Cable.**
→ Uruchom `EchoWarp doctor` — powie Ci dokładnie, co zainstalować i jak to zrobić. Na Linuksie dodatkowe oprogramowanie nie jest potrzebne.

**Urządzenie loopback nie pojawia się w TUI.**
→ macOS: Najpierw zainstaluj BlackHole (`brew install blackhole-2ch`). Windows/Linux: urządzenia loopback są wbudowane i powinny pojawiać się automatycznie.

**Urządzenie wirtualne wyświetla „adaptive" zamiast częstotliwości próbkowania.**
→ To normalne. Wirtualne sterowniki audio (BlackHole, VB-Cable) dostosowują się do częstotliwości próbkowania używanej przez aplikację — wyświetlana wartość nie ma znaczenia.

## Tryb CLI

Wszystkie ustawienia dostępne w TUI można również przekazać jako flagi CLI na potrzeby skryptów i automatyzacji:

```bash
# Serwer
EchoWarp server -d 1 -P mypassword

# Klient (automatycznie wykrywa serwer w sieci LAN, jeśli pominięto -a)
EchoWarp client -a 192.168.1.10 -d 2 -P mypassword

# Lista urządzeń audio
EchoWarp devices
```

### Tryby

Szczegółowe opisy poszczególnych trybów znajdziesz w sekcji [Tryby strumieniowania](#tryby-strumieniowania).

| Tryb | Flaga |
|------|------|
| Normalny | *(domyślnie)* |
| Odwrotny | `-r` |
| Dupleks | `-X` |
| Konferencja | `--conference` |

### Popularne flagi

| Flaga | Skrót | Opis |
|------|-------|-------------|
| `--device` | `-d` | ID urządzenia audio |
| `--device-name` | `-D` | Wybierz urządzenie po nazwie (dopasowanie podciągu) |
| `--password` | `-P` | Hasło uwierzytelniające |
| `--port` | `-p` | Port TCP (domyślnie: 4415) |
| `--sample-rate` | | Częstotliwość próbkowania (domyślnie: 48000) |
| `--channels` | | 1=mono, 2=stereo (domyślnie: 1) |
| `--max-clients` | | Maks. liczba połączonych klientów (domyślnie: 1) |
| `--virtual-mic` | | Utwórz wirtualne urządzenie mikrofonowe |
| `--config` | `-c` | Wczytaj plik konfiguracyjny YAML |
| `--save-config` | `-s` | Zapisz bieżące ustawienia do YAML |
| `--profile` | | Wczytaj zapisany profil urządzenia |
| `--stun-server` | | Niestandardowe serwery STUN do przechodzenia przez NAT |
| `--tls-cert` / `--tls-key` | | Certyfikat i klucz TLS |
| `--log-level` | | debug, info, warn, error |
| `--aec` | | Akustyczne tłumienie echa (dupleks) |
| `--loopback` | | Przechwytuj dźwięk systemowy (macOS, wymaga BlackHole) |
| `--dry-run` | | Sprawdź poprawność konfiguracji i wyjdź |

### Pliki konfiguracyjne

Priorytety ustawień: flagi CLI > zmienne środowiskowe (`ECHOWARP_*`) > plik konfiguracyjny > wartości domyślne.

Ekran konfiguracji TUI umożliwia interaktywne zapisywanie i wczytywanie plików konfiguracyjnych. Można również użyć flag CLI:

```bash
# Zapisz bieżące ustawienia do pliku
EchoWarp server -d 1 -P pass --channels 2 -s myconfig.yml

# Wczytaj ustawienia z pliku
EchoWarp server -c myconfig.yml
```

## Instalacja

### Gotowe pliki binarne

Pobierz ze strony [Releases](https://github.com/lHumaNl/EchoWarp/releases). Dostępne dla:
- **macOS** — arm64 (Apple Silicon), amd64 (Intel), zawiera pakiet `.app`
- **Linux** — amd64, arm64, armv7, armv6
- **Windows** — amd64, arm64

### Kompilacja ze źródeł

Wymagane: Go 1.22+ oraz nagłówki deweloperskie libopus.

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

Wynikowy plik binarny statycznie linkuje opus — brak zależności w czasie wykonania.

## Wymagania systemowe

- **macOS** 11.0+ (arm64, amd64)
- **Linux** (amd64, arm64, armv7, armv6)
- **Windows** 10+ (amd64, arm64)

Dla wirtualnego mikrofonu:
- **macOS**: sterownik audio [BlackHole](https://github.com/ExistentialAudio/BlackHole)
- **Linux**: PulseAudio (`pactl`)

## Sieć & Zapora sieciowa

### Serwer — porty do otwarcia

| Port | Protokół | Przeznaczenie |
|------|----------|---------------|
| `4415` | **TCP** | Sygnalizacja (uwierzytelnianie + uzgadnianie WebRTC) |
| `4415` | **UDP** | Media audio (WebRTC). Multipleksowane — jeden port obsługuje wszystkich klientów |
| `4416` | **TCP** | Sonda informacji o sesji (autokonfiguracja klienta) |

> Port `4415` jest domyślny i można go zmienić za pomocą `--port`. Port informacji o sesji to zawsze `port + 1`.

**W skrócie:** otwórz **TCP 4415–4416** i **UDP 4415** dla połączeń przychodzących na serwerze.

### Klient — nie wymaga portów przychodzących

Klient nawiązuje jedynie połączenia wychodzące. Po stronie klienta nie są potrzebne żadne reguły zapory ani przekierowania portów.

### Wykrywanie w sieci lokalnej

Jeśli korzystasz z automatycznego wykrywania mDNS (`Ctrl+F` w konfiguracji), zezwól na **multicast UDP na porcie 5353**. Można to wyłączyć za pomocą `--no-discovery`.

### Przechodzenie przez NAT (STUN / TURN)

EchoWarp używa serwerów STUN do nawiązywania połączeń przez NAT. Jeśli obie strony znajdują się za symetrycznym NAT i nie można nawiązać bezpośredniego połączenia, można skonfigurować serwer przekaźnikowy TURN (`turn_servers` w konfiguracji). Zarówno STUN, jak i TURN to połączenia **wychodzące** i nie wymagają żadnych reguł zapory dla ruchu przychodzącego.

## Licencja

[MIT](../../LICENSE) — zobacz [THIRD_PARTY_NOTICES.md](../../THIRD_PARTY_NOTICES.md), aby zapoznać się z licencjami stron trzecich.
