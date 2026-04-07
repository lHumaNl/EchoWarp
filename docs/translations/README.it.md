<p align="center">
  <img src="../../build/icons/icon_512.png" alt="EchoWarp Logo" width="128">
</p>

<h1 align="center">EchoWarp</h1>

<p align="center">
  Streaming audio in rete in tempo reale tra host
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

Acquisisci l'audio su una macchina e riproducilo su un'altra — in tempo reale attraverso la rete. EchoWarp utilizza WebRTC per il trasporto e Opus per la compressione, offrendo audio a bassa latenza con crittografia end-to-end.

## Funzionalità

- **Streaming 1:1** — il server acquisisce, il client riproduce (o viceversa)
- **Modalità duplex** — audio bidirezionale, entrambi i lati si sentono
- **Broadcast (1:N)** — un server trasmette a più client
- **Conferenza (N:N)** — mixaggio multi-partecipante con mix personali (totale meno sé stessi)
- **Microfono virtuale** — l'audio ricevuto appare come ingresso microfono (Discord, Zoom, OBS)
- **Rilevamento LAN** — rilevamento automatico del server tramite mDNS
- **Registrazione** — registrazione WAV: mixata, per traccia singola, o entrambe
- **Accelerazione SIMD** — AVX/SSE (x86) e NEON (ARM) per il mixaggio audio
- **Crittografia end-to-end** — AES + DTLS/SRTP
- **Multi-piattaforma** — macOS, Linux, Windows

## Avvio Rapido

1. Scarica un binario precompilato dalla pagina [Releases](https://github.com/lHumaNl/EchoWarp/releases) (nessuna dipendenza — opus è collegato staticamente)
2. Esegui `EchoWarp` — un menu interattivo permette di scegliere Server, Client, Diagnostica e altro. Oppure passa direttamente a una modalità con `EchoWarp server` / `EchoWarp client`
3. Configura tutto nella TUI interattiva e premi Invio per avviare

> **Utenti Windows:** L'archivio della release include `EchoWarp Server.bat` e `EchoWarp Client.bat` — fai doppio clic su quello che ti serve. Non è necessario aprire cmd.exe manualmente.

## TUI Interattiva

La TUI è l'interfaccia principale di EchoWarp. Esegui semplicemente `EchoWarp server` o `EchoWarp client` — la schermata di configurazione interattiva si apre automaticamente.

### Schermata di Configurazione

La schermata di configurazione ha un layout a due colonne:

**Colonna sinistra — Dispositivi audio.** Elenca tutti i dispositivi di ingresso/uscita disponibili con le relative proprietà (canali, frequenza di campionamento, profondità di bit). In modalità duplex o conferenza si assegnano ruoli separati di acquisizione `[C]` e riproduzione `[P]`. La creazione di dispositivi virtuali è disponibile direttamente dall'elenco.

**Colonna destra — Impostazioni.** Tutti i parametri di sessione sono configurabili qui con validazione in linea:

| Impostazione | Descrizione |
|---------|-------------|
| Indirizzo server | IP/hostname del server (solo client) |
| Porta | Porta TCP (predefinita: 4415) |
| Password | Password di autenticazione |
| Modalità | Normale / Inversa / Duplex |
| Max client | Connessioni massime (solo server) |
| Frequenza di campionamento | 8000 / 16000 / 24000 / 48000 Hz |
| Canali | Mono / Stereo |
| Bitrate Opus | 16–512 kbps |
| Cancellazione eco | AEC per la modalità duplex |
| TLS | Abilita/disabilita con percorsi certificato e chiave |
| Livello log | debug / info / warn / error |

Naviga tra le colonne con `Tab`, spostati tra i campi con i tasti freccia e premi `Invio` per confermare e avviare lo streaming.

### Rilevamento LAN

Nella schermata di configurazione del client, premi `Ctrl+F` per aprire il pannello di rilevamento. EchoWarp utilizza mDNS per trovare automaticamente i server nella rete locale — selezionane uno dall'elenco e i campi indirizzo/porta vengono compilati automaticamente.

### Profili Dispositivo

Premi `Ctrl+O` per aprire il pannello dei profili. Salva la configurazione corrente dei dispositivi come profilo con un nome, carica un profilo salvato in precedenza, o elimina i profili non più necessari. I profili ricordano le assegnazioni e i ruoli dei dispositivi.

### Schermata di Streaming

Una volta connesso, la TUI mostra lo stato dello streaming in tempo reale:

- **Statistiche di connessione** — RTT, jitter, perdita di pacchetti con barre di soglia che mostrano la vicinanza ai limiti di qualità
- **Qualità della connessione** — indicatore complessivo (Eccellente / Buona / Discreta / Scarsa) calcolato da RTT, jitter e perdita
- **Analizzatore di spettro** — visualizzazione FFT in tempo reale delle frequenze del segnale audio
- **Indicatori VU** — misuratori del livello audio per canale
- **Bitrate** — visualizzazione live del bitrate di upload/download
- **Controlli dispositivo** — regolazione volume, silenziamento, cambio dispositivo
- **Pannello log** — vista log scorrevole attivabile/disattivabile

### Scorciatoie Tastiera TUI

| Tasto | Azione |
|-----|--------|
| `Tab` | Cambia colonna (configurazione) |
| `Ctrl+F` | Rilevamento LAN (configurazione client) |
| `Ctrl+O` | Profili dispositivo (configurazione) |
| `Enter` | Conferma e avvia |
| `Ctrl+Q` | Esci |
| `Ctrl+P` | Metti in pausa/riprendi lo streaming |
| `Ctrl+L` | Attiva/disattiva il pannello log |
| `Ctrl+M` | Silenzia/riattiva |
| `+` / `-` | Alza/abbassa volume |
| `Ctrl+Up/Down` | Scorri i log |
| `Ctrl+R` | Avvia/ferma la registrazione |
| `Ctrl+D` | Espelli client (server) |
| `Ctrl+B` | Banna client (server) |

## Modalità di Streaming

EchoWarp ha quattro modalità di streaming. Scegli quella adatta al tuo scenario — la modalità viene selezionata nella schermata di configurazione della TUI (lato server) o rilevata automaticamente (lato client).

### Normale — Unidirezionale: Server verso Client

Il server acquisisce l'audio e lo invia a tutti i client connessi. I client si limitano ad ascoltare.

```
Server [acquisisce audio] ───→ Client 1 [riproduce audio]
                          ├──→ Client 2 [riproduce audio]
                          └──→ Client N [riproduce audio]
```

**Quando usarla:** trasmettere musica/podcast agli ascoltatori, inviare l'audio di sistema (YouTube, Spotify) in un'altra stanza, inviare un feed loopback a una macchina remota.

**Dispositivi necessari:**

| Lato | Acquisizione (input) | Riproduzione (output) |
|------|:-:|:-:|
| Server | richiesto | — |
| Client | — | richiesto |

**Partecipanti:** 1 server + 1…N client (imposta *Max client* nella TUI).

---

### Inversa — Unidirezionale: Client verso Server

L'opposto della modalità Normale. I client acquisiscono l'audio e lo inviano al server. Il server si limita ad ascoltare.

```
Client 1 [acquisisce audio] ───→ Server [riproduce audio]
Client 2 [acquisisce audio] ──┘
```

**Quando usarla:** utilizzare un microfono remoto sulla macchina server, raccogliere audio da una posizione remota, inviare il microfono del client agli altoparlanti del server.

**Dispositivi necessari:**

| Lato | Acquisizione (input) | Riproduzione (output) |
|------|:-:|:-:|
| Server | — | richiesto |
| Client | richiesto | — |

**Partecipanti:** 1 server + 1…N client.

---

### Duplex — Bidirezionale

Entrambi i lati acquisiscono e riproducono simultaneamente. Tutti sentono tutti.

```
Server [microfono + altoparlanti] ⟷ Client 1 [microfono + altoparlanti]
                                  ⟷ Client 2 [microfono + altoparlanti]
```

**Quando usarla:** chiamata vocale tra due o più macchine, interfono tra stanze, sessione audio collaborativa.

**Dispositivi necessari:**

| Lato | Acquisizione (input) | Riproduzione (output) |
|------|:-:|:-:|
| Server | richiesto | richiesto |
| Client | richiesto | richiesto |

**Partecipanti:** 1 server + 1…N client. Supporta AEC (cancellazione eco) per prevenire loop di feedback.

---

### Conferenza — Mixaggio Multi-Utente (N:N)

Il server funge da hub di mixaggio. Ogni partecipante (server + client) invia il proprio audio e riceve un mix personale di **tutti gli altri** (totale meno sé stesso). Questo evita di sentire la propria voce.

```
Client 1 ──┐              ┌──→ Client 1 (sente Server + 2 + 3)
Client 2 ──┤  Server hub  ├──→ Client 2 (sente Server + 1 + 3)
Client 3 ──┘              └──→ Client 3 (sente Server + 1 + 2)
```

**Quando usarla:** chiamate di gruppo con 3+ macchine, prove da remoto, conferenze multi-partecipante.

**Dispositivi necessari:**

| Lato | Acquisizione (input) | Riproduzione (output) |
|------|:-:|:-:|
| Server (solo hub) | — | — |
| Server (partecipante) | richiesto | richiesto |
| Client | richiesto | richiesto |

Il server può funzionare come **solo hub** (nessun audio locale — solo mixaggio per i client) o come partecipante a tutti gli effetti con microfono e altoparlanti propri.

**Partecipanti:** 1 server + 2…N client. Supporta registrazione (mixata, per traccia singola, o entrambe), espulsione/ban e controllo volume per partecipante.

---

### Riepilogo delle Modalità

| | Normale | Inversa | Duplex | Conferenza |
|---|---|---|---|---|
| **Direzione** | Server → Client | Client → Server | Entrambe le direzioni | Tutti ⟷ Tutti |
| **Dispositivi server** | Solo acquisizione | Solo riproduzione | Entrambi | Entrambi (o nessuno se hub) |
| **Dispositivi client** | Solo riproduzione | Solo acquisizione | Entrambi | Entrambi |
| **Max partecipanti** | 1 + N | 1 + N | 1 + N | 1 + N |
| **Supporto AEC** | — | — | sì | sì |
| **Mix personale** | — | — | — | sì |
| **Registrazione** | — | — | — | sì |
| **Flag CLI** | *(predefinito)* | `-r` | `-X` | `--conference` |

> **Suggerimento:** Il numero di client è controllato dall'impostazione *Max client* nella TUI (predefinito: 1). Aumentalo se hai bisogno di scenari broadcast o multi-utente.

## Guida al Routing Audio

### Casi d'uso

EchoWarp supporta diversi scenari di routing audio. La tabella seguente mostra quale modalità e configurazione dei dispositivi utilizzare per ciascuno:

| Scenario | Modalità | Dispositivi server | Dispositivi client |
|----------|----------|--------------------|--------------------|
| Trasmetti il microfono agli altoparlanti remoti | Normal | Microfono `[Input]` | Altoparlanti `[Output]` |
| Trasmetti l'audio di sistema (YouTube, Spotify) | Normal | 🔄 Loopback `[Input]` | Altoparlanti `[Output]` |
| Usa il microfono remoto in Discord/Zoom | Reverse | Altoparlanti `[Output]` | Microfono `[Input]` |
| Chat vocale bidirezionale | Duplex | Microfono + Altoparlanti | Microfono + Altoparlanti |
| Chiamata di gruppo (3+ macchine) | Conference | — (hub) | Microfono + Altoparlanti |
| Audio remoto come microfono virtuale in OBS | Normal | 🔄 Loopback `[Input]` | ⟡ Microfono virtuale `[Output]` |
| Stream + microfono locale come unico microfono virtuale | Normal | Microfono `[Input]` | ⟡ Virtuale `[Output]` + 🎤 Mix mic |

### Loopback — Cattura l'audio di sistema

I dispositivi loopback catturano tutto l'audio riprodotto su una macchina (musica, videochiamate, audio di gioco) e lo rendono disponibile come sorgente di input.

| Piattaforma | Come funziona | Configurazione |
|-------------|---------------|----------------|
| **macOS** | Aggregate Device (altoparlanti + BlackHole) | Installa [BlackHole](https://github.com/ExistentialAudio/BlackHole): `brew install blackhole-2ch`. I dispositivi loopback appaiono automaticamente nella TUI |
| **Windows** | Loopback nativo WASAPI | Integrato, nessun software aggiuntivo necessario |
| **Linux** | Sorgenti monitor PulseAudio | Integrato con PulseAudio/PipeWire |

Nella TUI, i dispositivi loopback sono contrassegnati con 🔄 e appaiono nella sezione Input.

### Microfono virtuale — Instrada l'audio verso altre app

Un microfono virtuale fa apparire l'audio ricevuto come input del microfono che Discord, Zoom, OBS e altre applicazioni possono utilizzare.

| Piattaforma | Driver | Installazione |
|-------------|--------|---------------|
| **macOS** | [BlackHole](https://github.com/ExistentialAudio/BlackHole) | `brew install blackhole-2ch` |
| **Windows** | [VB-Audio Virtual Cable](https://vb-audio.com/Cable/) | Scarica da vb-audio.com |
| **Linux** | Null-sink PulseAudio | Creato dalla TUI: Impostazioni → Microfono virtuale → Crea |

Su macOS e Windows, installa il driver e selezionalo nella sezione Output della TUI. Su Linux, la TUI crea automaticamente il sink virtuale — nelle altre app, seleziona **"Monitor of EchoWarp"** come microfono.

I dispositivi virtuali sono contrassegnati con ⟡ nella TUI e mostrano **adaptive** invece di una frequenza di campionamento fissa.

### Mixaggio del microfono locale nell'uscita virtuale

Quando instradi uno stream remoto verso un dispositivo di uscita virtuale (BlackHole, VB-Cable), app di terze parti come Discord o Zoom vedono solo lo stream — non sentono il tuo microfono locale. Se hai bisogno sia della tua voce **che** dello stream come un unico ingresso microfono, EchoWarp può mixarli insieme.

**Come usarlo:** Nella TUI, seleziona un dispositivo virtuale nella sezione Output. I dispositivi di input appariranno sotto come sotto-elementi — spunta il microfono che vuoi mixare:

```
🔊 Output
  ⟡ BlackHole 2ch                           stereo 32-bit adaptive   [✓]
      └─ 🎤 Built-in Microphone                                      [✓]
```

L'audio mixato (stream + microfono) viene scritto sull'uscita virtuale. In Discord/Zoom/OBS, seleziona quel dispositivo virtuale come microfono — sia lo stream che la tua voce saranno udibili.

**Alternative a livello di sistema operativo** (senza utilizzare il mixer integrato):

| Piattaforma | Come | Dettagli |
|-------------|------|----------|
| **macOS** | Aggregate Device | Apri Configurazione Audio MIDI → Crea un dispositivo aggregato combinando il tuo microfono + BlackHole. Seleziona l'aggregato come input nella tua app |
| **Windows** | [VoiceMeeter](https://vb-audio.com/Voicemeeter/) | Mixer virtuale gratuito — instradi sia VB-Cable che il tuo microfono in VoiceMeeter, usa la sua uscita come microfono nella tua app |
| **Linux** | PulseAudio combine-source | `pactl load-module module-combine-source source_name=combined slaves=<mic>,<virtual>.monitor` |

Il mixer integrato funziona su tutte le piattaforme e non richiede configurazione aggiuntiva.

### Sezioni dei dispositivi

La TUI mostra solo le sezioni rilevanti per la modalità corrente:

| Modalità + Lato | Sezioni visibili |
|-----------------|-----------------|
| Server Normal / Client Reverse | Solo 🎤 Input |
| Client Normal / Server Reverse | Solo 🔊 Output |
| Duplex / Conference | 🎤 Input + 🔊 Output |

### Diagnostica

Esegui `EchoWarp doctor` per verificare la configurazione audio:

```
Audio
[OK] Audio system: 2 input device(s), 2 output device(s)
[OK] Virtual audio: BlackHole 2ch detected
```

Se non viene trovato alcun driver audio virtuale, il doctor mostrerà le istruzioni di installazione per la tua piattaforma.

### FAQ

**Voglio ascoltare la musica dal mio computer sugli altoparlanti in un'altra stanza.**
→ Esegui `EchoWarp server` sulla macchina con la musica, seleziona il dispositivo 🔄 loopback (i tuoi altoparlanti) in Input. Esegui `EchoWarp client` sulla macchina remota, seleziona gli altoparlanti in Output.

**Voglio usare un microfono remoto per Zoom/Discord.**
→ Esegui `EchoWarp server` in modalità Reverse sulla macchina remota (quella con il microfono). Esegui `EchoWarp client` sulla tua macchina, seleziona un dispositivo audio virtuale (BlackHole/VB-Cable) in Output. In Zoom/Discord, seleziona quel dispositivo virtuale come microfono.

**Voglio trasmettere l'audio di sistema (YouTube, Spotify) a un'altra macchina.**
→ Come "musica in un'altra stanza" — usa 🔄 loopback sul lato server. Il dispositivo loopback cattura tutto ciò che viene riprodotto attraverso gli altoparlanti selezionati.

**Voglio che l'audio remoto appaia come input del microfono in OBS.**
→ Avvia il server con la sorgente audio. Sulla tua macchina (client), seleziona un dispositivo audio virtuale in Output. In OBS, aggiungi una sorgente "Audio Input Capture" e seleziona il dispositivo virtuale.

**Voglio una chat vocale bidirezionale tra due computer.**
→ Usa la modalità Duplex. Su entrambe le macchine, seleziona un microfono in Input e gli altoparlanti in Output.

**Voglio una chiamata di gruppo con 3+ macchine.**
→ Usa la modalità Conference. Il server funge da hub (nessun dispositivo necessario). Ogni client seleziona un microfono e gli altoparlanti. Tutti sentono tutti gli altri, tranne la propria voce.

**Uso Moonlight/Sunshine (o NVIDIA GameStream) e voglio che il mio microfono funzioni nei giochi sull'host.**
→ Avvia `EchoWarp server` in modalità Reverse sull'host di gioco (macchina Sunshine/GameStream). Avvia `EchoWarp client` sulla macchina Moonlight, seleziona il tuo microfono in Input. Sul server, seleziona un dispositivo audio virtuale (BlackHole/VB-Cable) in Output, oppure abilita `--virtual-mic` per crearne uno automaticamente. Nel tuo gioco o chat vocale sull'host, seleziona quel dispositivo virtuale come microfono. La tua voce dal client Moonlight apparirà come ingresso microfonico sull'host di gioco.

**Voglio che Discord senta sia lo stream remoto CHE la mia voce attraverso un unico microfono virtuale.**
→ Sul client, seleziona un dispositivo virtuale (BlackHole/VB-Cable) in Output. Sotto apparirà un elenco di dispositivi di input — spunta il tuo microfono. EchoWarp mixerà lo stream e il tuo microfono nell'uscita virtuale. In Discord, seleziona il dispositivo virtuale come microfono.

**Non ho installato BlackHole / VB-Cable.**
→ Esegui `EchoWarp doctor` — ti dirà esattamente cosa installare e come. Su Linux non è necessario software aggiuntivo.

**Il dispositivo loopback non appare nella TUI.**
→ macOS: installa prima BlackHole (`brew install blackhole-2ch`). Windows/Linux: i dispositivi loopback sono integrati e dovrebbero apparire automaticamente.

**Il dispositivo virtuale mostra "adaptive" invece di una frequenza di campionamento.**
→ È normale. I driver audio virtuali (BlackHole, VB-Cable) si adattano alla frequenza di campionamento utilizzata dall'applicazione — la frequenza visualizzata non è significativa.

<details>
<summary>macOS: "Impossibile aprire EchoWarp" / avviso Gatekeeper</summary>

macOS blocca le applicazioni non firmate. Per consentire l'esecuzione di EchoWarp:

```bash
xattr -cr /path/to/EchoWarp       # per il binario
xattr -cr /path/to/EchoWarp.app   # per il bundle .app
```

In alternativa: **Impostazioni di Sistema → Privacy e Sicurezza → "Consenti comunque"**

</details>

## Modalità CLI

Tutte le impostazioni disponibili nella TUI possono essere passate anche come flag CLI per scripting e automazione:

```bash
# Server
EchoWarp server -d 1 -P mypassword

# Client (rileva automaticamente il server sulla LAN se -a è omesso)
EchoWarp client -a 192.168.1.10 -d 2 -P mypassword

# Elenca i dispositivi audio
EchoWarp devices
```

### Modalità

Vedi [Modalità di Streaming](#modalità-di-streaming) per descrizioni dettagliate di ogni modalità.

| Modalità | Flag |
|------|------|
| Normale | *(predefinito)* |
| Inversa | `-r` |
| Duplex | `-X` |
| Conferenza | `--conference` |

### Flag Comuni

| Flag | Breve | Descrizione |
|------|-------|-------------|
| `--device` | `-d` | ID dispositivo audio |
| `--device-name` | `-D` | Seleziona dispositivo per nome (corrispondenza sottostringa) |
| `--password` | `-P` | Password di autenticazione |
| `--port` | `-p` | Porta TCP (predefinita: 4415) |
| `--sample-rate` | | Frequenza di campionamento (predefinita: 48000) |
| `--channels` | | 1=mono, 2=stereo (predefinito: 1) |
| `--max-clients` | | Max client connessi (predefinito: 1) |
| `--virtual-mic` | | Crea un dispositivo microfono virtuale |
| `--config` | `-c` | Carica file di configurazione YAML |
| `--save-config` | `-s` | Salva le impostazioni correnti in YAML |
| `--profile` | | Carica un profilo dispositivo salvato |
| `--stun-server` | | Server STUN personalizzati per l'attraversamento NAT |
| `--tls-cert` / `--tls-key` | | Certificato e chiave TLS |
| `--log-level` | | debug, info, warn, error |
| `--aec` | | Cancellazione eco acustica (duplex) |
| `--loopback` | | Acquisisci audio di sistema (macOS, richiede BlackHole) |
| `--dry-run` | | Valida la configurazione ed esci |

### File di Configurazione

Priorità delle impostazioni: flag CLI > variabili d'ambiente (`ECHOWARP_*`) > file di configurazione > valori predefiniti.

La schermata di configurazione del TUI consente di salvare e caricare file di configurazione in modo interattivo. È possibile anche utilizzare i flag CLI:

```bash
# Salva le impostazioni correnti in un file
EchoWarp server -d 1 -P pass --channels 2 -s myconfig.yml

# Carica le impostazioni da un file
EchoWarp server -c myconfig.yml
```

## Installazione

### Binari Precompilati

Scarica dalla pagina [Releases](https://github.com/lHumaNl/EchoWarp/releases). Disponibile per:
- **macOS** — arm64 (Apple Silicon), amd64 (Intel), include bundle `.app`
- **Linux** — amd64, arm64, armv7, armv6
- **Windows** — amd64, arm64

### Compilazione dal Sorgente

Richiede Go 1.22+ e gli header di sviluppo di libopus.

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

Il binario risultante collega opus staticamente — nessuna dipendenza a runtime necessaria.

## Requisiti di Sistema

- **macOS** 11.0+ (arm64, amd64)
- **Linux** (amd64, arm64, armv7, armv6)
- **Windows** 10+ (amd64, arm64)

Per il microfono virtuale:
- **macOS**: driver audio [BlackHole](https://github.com/ExistentialAudio/BlackHole)
- **Windows**: [VB-Audio Virtual Cable](https://vb-audio.com/Cable/)
- **Linux**: PulseAudio (`pactl`)

## Rete & Firewall

### Server — porte da aprire

| Porta | Protocollo | Scopo |
|-------|------------|-------|
| `4415` | **TCP** | Segnalazione (autenticazione + handshake WebRTC) |
| `4415` | **UDP** | Media audio (WebRTC). Multiplexato — una sola porta gestisce tutti i client |
| `4416` | **TCP** | Sonda informazioni sessione (auto-configurazione del client) |

> La porta `4415` è quella predefinita e può essere modificata con `--port`. La porta per le informazioni di sessione è sempre `port + 1`.

**In breve:** aprire **TCP 4415–4416** e **UDP 4415** in ingresso sul server.

### Client — nessuna porta in ingresso necessaria

Il client effettua solo connessioni in uscita. Non sono necessarie regole firewall o port-forwarding sul lato client.

### Scoperta nella rete locale

Se si utilizza la scoperta automatica via mDNS (`Ctrl+F` durante la configurazione), consentire il **multicast UDP sulla porta 5353**. Questa funzione può essere disattivata con `--no-discovery`.

### Attraversamento NAT (STUN / TURN)

EchoWarp utilizza server STUN per stabilire connessioni attraverso il NAT. Se entrambi i lati si trovano dietro un NAT simmetrico e non è possibile stabilire una connessione diretta, è possibile configurare un server relay TURN (`turn_servers` nel file di configurazione). Sia STUN che TURN sono connessioni **in uscita** e non richiedono alcuna regola firewall in ingresso.

## Licenza

[MIT](../../LICENSE) — vedi [THIRD_PARTY_NOTICES.md](../../THIRD_PARTY_NOTICES.md) per le licenze di terze parti.
