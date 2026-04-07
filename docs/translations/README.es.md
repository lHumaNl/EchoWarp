<p align="center">
  <img src="../../build/icons/icon_512.png" alt="EchoWarp Logo" width="128">
</p>

<h1 align="center">EchoWarp</h1>

<p align="center">
  Transmisión de audio en red en tiempo real entre hosts
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

Captura audio en una máquina y reprodúcelo en otra — en tiempo real a través de la red. EchoWarp utiliza WebRTC para el transporte y Opus para la compresión, ofreciendo audio de baja latencia con cifrado de extremo a extremo.

## Características

- **Transmisión 1:1** — el servidor captura, el cliente reproduce (o a la inversa)
- **Modo dúplex** — audio bidireccional, ambos lados se escuchan mutuamente
- **Difusión (1:N)** — un servidor transmite a múltiples clientes
- **Conferencia (N:N)** — mezcla multiparte con mezclas personales (total menos uno mismo)
- **Micrófono virtual** — el audio recibido aparece como entrada de micrófono (Discord, Zoom, OBS)
- **Descubrimiento en LAN** — detección automática de servidores mediante mDNS
- **Grabación** — grabación WAV: mezclada, por pista, o ambas
- **Aceleración SIMD** — AVX/SSE (x86) y NEON (ARM) para la mezcla de audio
- **Cifrado de extremo a extremo** — AES + DTLS/SRTP
- **Multiplataforma** — macOS, Linux, Windows

## Inicio rápido

1. Descarga un binario precompilado desde [Releases](https://github.com/lHumaNl/EchoWarp/releases) (sin dependencias — opus está enlazado estáticamente)
2. Ejecute `EchoWarp` — un menú interactivo permite elegir Servidor, Cliente, Diagnóstico y más. O salte directamente a un modo con `EchoWarp server` / `EchoWarp client`
3. Configura todo en la TUI interactiva y pulsa Enter para iniciar

> **Usuarios de Windows:** El archivo de la release incluye `EchoWarp Server.bat` y `EchoWarp Client.bat` — haz doble clic en el que necesites. No es necesario abrir cmd.exe manualmente.

## TUI interactiva

La TUI es la interfaz principal de EchoWarp. Simplemente ejecuta `EchoWarp server` o `EchoWarp client` — la pantalla de configuración interactiva se abre automáticamente.

### Pantalla de configuración

La pantalla de configuración tiene un diseño de dos columnas:

**Columna izquierda — Dispositivos de audio.** Lista todos los dispositivos de entrada/salida disponibles con sus propiedades (canales, frecuencia de muestreo, profundidad de bits). En modo dúplex o conferencia puedes asignar roles separados de captura `[C]` y reproducción `[P]`. La creación de dispositivos virtuales está disponible directamente desde la lista.

**Columna derecha — Ajustes.** Todos los parámetros de sesión son configurables aquí con validación en línea:

| Ajuste | Descripción |
|--------|-------------|
| Dirección del servidor | IP/hostname del servidor (solo cliente) |
| Puerto | Puerto TCP (por defecto: 4415) |
| Contraseña | Contraseña de autenticación |
| Modo | Normal / Inverso / Dúplex |
| Máx. clientes | Conexiones máximas (solo servidor) |
| Frecuencia de muestreo | 8000 / 16000 / 24000 / 48000 Hz |
| Canales | Mono / Estéreo |
| Bitrate Opus | 16–512 kbps |
| Cancelación de eco | AEC para modo dúplex |
| TLS | Activar/desactivar con rutas de certificado y clave |
| Nivel de registro | debug / info / warn / error |

Navega entre columnas con `Tab`, muévete por los campos con las teclas de flecha y pulsa `Enter` para confirmar e iniciar la transmisión.

### Descubrimiento en LAN

En la pantalla de configuración del cliente, pulsa `Ctrl+F` para abrir el panel de descubrimiento. EchoWarp utiliza mDNS para encontrar automáticamente servidores en la red local — selecciona uno de la lista y los campos de dirección/puerto se rellenan automáticamente.

### Perfiles de dispositivo

Pulsa `Ctrl+O` para abrir el panel de perfiles. Guarda la configuración actual de dispositivos como un perfil con nombre, carga uno guardado anteriormente o elimina perfiles que ya no necesites. Los perfiles recuerdan las asignaciones y roles de los dispositivos.

### Pantalla de transmisión

Una vez conectada, la TUI muestra el estado de la transmisión en tiempo real:

- **Estadísticas de conexión** — RTT, jitter, pérdida de paquetes con barras de umbral que muestran la proximidad a los límites de calidad
- **Calidad de conexión** — indicador general (Excelente / Buena / Regular / Deficiente) calculado a partir del RTT, jitter y pérdida
- **Analizador de espectro** — visualización FFT de frecuencias en tiempo real de la señal de audio
- **Medidores VU** — medidores de nivel de audio por canal
- **Bitrate** — visualización en vivo del bitrate de subida/bajada
- **Controles de dispositivo** — ajuste de volumen, silencio, cambio de dispositivo
- **Panel de registro** — vista de registro desplazable con alternancia

### Atajos de teclado de la TUI

| Tecla | Acción |
|-------|--------|
| `Tab` | Cambiar columnas (configuración) |
| `Ctrl+F` | Descubrimiento en LAN (configuración del cliente) |
| `Ctrl+O` | Perfiles de dispositivo (configuración) |
| `Enter` | Confirmar e iniciar |
| `Ctrl+Q` | Salir |
| `Ctrl+P` | Pausar/reanudar transmisión |
| `Ctrl+L` | Alternar panel de registro |
| `Ctrl+M` | Silenciar/reactivar audio |
| `+` / `-` | Subir/bajar volumen |
| `Ctrl+Up/Down` | Desplazar registros |
| `Ctrl+R` | Iniciar/detener grabación |
| `Ctrl+D` | Expulsar cliente (servidor) |
| `Ctrl+B` | Banear cliente (servidor) |

## Modos de Transmisión

EchoWarp tiene cuatro modos de transmisión. Elige el que se ajuste a tu escenario — el modo se selecciona en la pantalla de configuración de la TUI (lado del servidor) o se detecta automáticamente (lado del cliente).

### Normal — Unidireccional: Servidor a Clientes

El servidor captura audio y lo envía a todos los clientes conectados. Los clientes solo escuchan.

```
Server [captures audio] ───→ Client 1 [plays audio]
                         ├──→ Client 2 [plays audio]
                         └──→ Client N [plays audio]
```

**Cuándo usarlo:** transmitir música/podcasts a oyentes, difundir audio del sistema (YouTube, Spotify) a otra habitación, enviar una fuente loopback a una máquina remota.

**Dispositivos necesarios:**

| Lado | Captura (entrada) | Reproducción (salida) |
|------|:-:|:-:|
| Servidor | necesario | — |
| Cliente | — | necesario |

**Participantes:** 1 servidor + 1…N clientes (configura *Máx. clientes* en la TUI).

---

### Inverso — Unidireccional: Clientes a Servidor

Lo opuesto a Normal. Los clientes capturan audio y lo envían al servidor. El servidor solo escucha.

```
Client 1 [captures audio] ───→ Server [plays audio]
Client 2 [captures audio] ──┘
```

**Cuándo usarlo:** usar un micrófono remoto en la máquina del servidor, recoger audio de una ubicación remota, enviar el micrófono de un cliente a los altavoces del servidor.

**Dispositivos necesarios:**

| Lado | Captura (entrada) | Reproducción (salida) |
|------|:-:|:-:|
| Servidor | — | necesario |
| Cliente | necesario | — |

**Participantes:** 1 servidor + 1…N clientes.

---

### Dúplex — Bidireccional

Ambos lados capturan y reproducen simultáneamente. Todos escuchan a todos.

```
Server [mic + speakers] ⟷ Client 1 [mic + speakers]
                        ⟷ Client 2 [mic + speakers]
```

**Cuándo usarlo:** llamada de voz entre dos o más máquinas, intercomunicador entre habitaciones, sesión de audio colaborativa.

**Dispositivos necesarios:**

| Lado | Captura (entrada) | Reproducción (salida) |
|------|:-:|:-:|
| Servidor | necesario | necesario |
| Cliente | necesario | necesario |

**Participantes:** 1 servidor + 1…N clientes. Compatible con AEC (cancelación de eco) para evitar bucles de retroalimentación.

---

### Conferencia — Mezcla Multiusuario (N:N)

El servidor actúa como un hub de mezcla. Cada participante (servidor + clientes) envía su audio y recibe una mezcla personal de **todos los demás** (total menos uno mismo). Esto evita escuchar tu propia voz.

```
Client 1 ──┐              ┌──→ Client 1 (hears Server + 2 + 3)
Client 2 ──┤  Server hub  ├──→ Client 2 (hears Server + 1 + 3)
Client 3 ──┘              └──→ Client 3 (hears Server + 1 + 2)
```

**Cuándo usarlo:** llamadas grupales con 3+ máquinas, ensayos remotos, conferencias con múltiples participantes.

**Dispositivos necesarios:**

| Lado | Captura (entrada) | Reproducción (salida) |
|------|:-:|:-:|
| Servidor (solo hub) | — | — |
| Servidor (participante) | necesario | necesario |
| Cliente | necesario | necesario |

El servidor puede funcionar como **solo hub** (sin audio local — solo mezcla para los clientes) o como participante completo con su propio micrófono y altavoces.

**Participantes:** 1 servidor + 2…N clientes. Compatible con grabación (mezclada, por pista, o ambas), expulsión/baneo y control de volumen por participante.

---

### Resumen de Modos

| | Normal | Inverso | Dúplex | Conferencia |
|---|---|---|---|---|
| **Dirección** | Servidor → Clientes | Clientes → Servidor | Ambas direcciones | Todos ⟷ Todos |
| **Dispositivos del servidor** | Solo captura | Solo reproducción | Ambos | Ambos (o ninguno si es hub) |
| **Dispositivos del cliente** | Solo reproducción | Solo captura | Ambos | Ambos |
| **Máx. participantes** | 1 + N | 1 + N | 1 + N | 1 + N |
| **Soporte AEC** | — | — | sí | sí |
| **Mezcla personal** | — | — | — | sí |
| **Grabación** | — | — | — | sí |
| **Flag CLI** | *(por defecto)* | `-r` | `-X` | `--conference` |

> **Consejo:** El número de clientes se controla con el ajuste *Máx. clientes* en la TUI (por defecto: 1). Auméntalo si necesitas difusión o escenarios multiusuario.

## Guía de Enrutamiento de Audio

### Casos de Uso

EchoWarp admite varios escenarios de enrutamiento de audio. La tabla a continuación muestra qué modo y configuración de dispositivos usar en cada caso:

| Escenario | Modo | Dispositivos del servidor | Dispositivos del cliente |
|-----------|------|---------------------------|--------------------------|
| Transmitir micrófono a altavoces remotos | Normal | Micrófono `[Input]` | Altavoces `[Output]` |
| Transmitir audio del sistema (YouTube, Spotify) | Normal | 🔄 Loopback `[Input]` | Altavoces `[Output]` |
| Usar micrófono remoto en Discord/Zoom | Reverse | Altavoces `[Output]` | Micrófono `[Input]` |
| Chat de voz bidireccional | Duplex | Micrófono + Altavoces | Micrófono + Altavoces |
| Llamada grupal (3+ máquinas) | Conference | — (hub) | Micrófono + Altavoces |
| Audio remoto como micrófono virtual en OBS | Normal | 🔄 Loopback `[Input]` | ⟡ Micrófono virtual `[Output]` |
| Stream + micrófono local como un solo micrófono virtual | Normal | Micrófono `[Input]` | ⟡ Virtual `[Output]` + 🎤 Mezclar mic |

### Loopback — Capturar Audio del Sistema

Los dispositivos loopback capturan todo el audio que se reproduce en una máquina (música, videollamadas, sonido de juegos) y lo hacen disponible como fuente de entrada.

| Plataforma | Funcionamiento | Configuración |
|------------|---------------|---------------|
| **macOS** | Dispositivo agregado (altavoces + BlackHole) | Instala [BlackHole](https://github.com/ExistentialAudio/BlackHole): `brew install blackhole-2ch`. Los dispositivos loopback aparecen en la TUI automáticamente |
| **Windows** | Loopback nativo WASAPI | Integrado, no se necesita software adicional |
| **Linux** | Fuentes monitor de PulseAudio | Integrado con PulseAudio/PipeWire |

En la TUI, los dispositivos loopback están marcados con 🔄 y aparecen en la sección Input.

### Micrófono Virtual — Enrutar Audio a Otras Aplicaciones

Un micrófono virtual hace que el audio recibido aparezca como entrada de micrófono que Discord, Zoom, OBS y otras aplicaciones pueden usar.

| Plataforma | Driver | Instalación |
|------------|--------|-------------|
| **macOS** | [BlackHole](https://github.com/ExistentialAudio/BlackHole) | `brew install blackhole-2ch` |
| **Windows** | [VB-Audio Virtual Cable](https://vb-audio.com/Cable/) | Descargar desde vb-audio.com |
| **Linux** | PulseAudio null-sink | Creado desde la TUI: Settings → Virtual mic → Create |

En macOS y Windows, instala el driver y selecciónalo en la sección Output de la TUI. En Linux, la TUI crea el sink virtual automáticamente — en otras aplicaciones, selecciona **"Monitor of EchoWarp"** como micrófono.

Los dispositivos virtuales están marcados con ⟡ en la TUI y muestran **adaptive** en lugar de una frecuencia de muestreo fija.

### Mezcla del Micrófono Local en la Salida Virtual

Cuando enrutas una transmisión remota a un dispositivo de salida virtual (BlackHole, VB-Cable), aplicaciones de terceros como Discord o Zoom solo escuchan la transmisión — no captan tu micrófono local. Si necesitas que tanto tu voz **como** la transmisión aparezcan como una sola entrada de micrófono, EchoWarp puede mezclarlos.

**Cómo usarlo:** En la TUI, selecciona un dispositivo virtual en la sección Output. Los dispositivos de entrada aparecerán debajo como subelementos — marca el micrófono que quieras mezclar:

```
🔊 Output
  ⟡ BlackHole 2ch                           stereo 32-bit adaptive   [✓]
      └─ 🎤 Built-in Microphone                                      [✓]
```

El audio mezclado (transmisión + micrófono) se escribe en la salida virtual. En Discord/Zoom/OBS, selecciona ese dispositivo virtual como tu micrófono — tanto la transmisión como tu voz se escucharán.

**Alternativas a nivel de sistema operativo** (sin usar el mezclador integrado):

| Plataforma | Cómo | Detalles |
|------------|------|----------|
| **macOS** | Dispositivo agregado | Abre Audio MIDI Setup → Crea un Dispositivo Agregado combinando tu micrófono + BlackHole. Selecciona el agregado como entrada en tu aplicación |
| **Windows** | [VoiceMeeter](https://vb-audio.com/Voicemeeter/) | Mezclador virtual gratuito — enruta tanto VB-Cable como tu micrófono a VoiceMeeter, usa su salida como micrófono en tu aplicación |
| **Linux** | PulseAudio combine-source | `pactl load-module module-combine-source source_name=combined slaves=<mic>,<virtual>.monitor` |

El mezclador integrado funciona en todas las plataformas y no requiere configuración adicional.

### Secciones de Dispositivos

La TUI muestra solo las secciones relevantes para el modo actual:

| Modo + Lado | Secciones visibles |
|-------------|-------------------|
| Servidor Normal / Cliente Reverse | 🎤 Solo Input |
| Cliente Normal / Servidor Reverse | 🔊 Solo Output |
| Duplex / Conference | 🎤 Input + 🔊 Output |

### Diagnóstico

Ejecuta `EchoWarp doctor` para verificar la configuración de tu audio:

```
Audio
[OK] Audio system: 2 input device(s), 2 output device(s)
[OK] Virtual audio: BlackHole 2ch detected
```

Si no se encuentra ningún driver de audio virtual, el doctor mostrará instrucciones de instalación para tu plataforma.

### Preguntas Frecuentes

**Quiero escuchar música de mi ordenador en altavoces de otra habitación.**
→ Ejecuta `EchoWarp server` en la máquina con música, selecciona el dispositivo 🔄 loopback (tus altavoces) en Input. Ejecuta `EchoWarp client` en la máquina remota, selecciona los altavoces en Output.

**Quiero usar un micrófono remoto para Zoom/Discord.**
→ Ejecuta `EchoWarp server` en modo Reverse en la máquina remota (la que tiene el micrófono). Ejecuta `EchoWarp client` en tu máquina, selecciona un dispositivo de audio virtual (BlackHole/VB-Cable) en Output. En Zoom/Discord, selecciona ese dispositivo virtual como micrófono.

**Quiero transmitir audio del sistema (YouTube, Spotify) a otra máquina.**
→ Igual que "música en otra habitación" — usa 🔄 loopback en el lado del servidor. El dispositivo loopback captura todo lo que se reproduce a través de los altavoces seleccionados.

**Quiero que el audio remoto aparezca como entrada de micrófono en OBS.**
→ Ejecuta el servidor con la fuente de audio. En tu máquina (cliente), selecciona un dispositivo de audio virtual en Output. En OBS, añade una fuente "Audio Input Capture" y selecciona el dispositivo virtual.

**Quiero un chat de voz bidireccional entre dos ordenadores.**
→ Usa el modo Duplex. En ambas máquinas, selecciona un micrófono en Input y altavoces en Output.

**Quiero una llamada grupal con 3+ máquinas.**
→ Usa el modo Conference. El servidor actúa como hub (no se necesitan dispositivos). Cada cliente selecciona un micrófono y altavoces. Todos escuchan a todos los demás, excepto su propia voz.

**Quiero que Discord escuche tanto la transmisión remota COMO mi voz a través de un solo micrófono virtual.**
→ En el cliente, selecciona un dispositivo virtual (BlackHole/VB-Cable) en Output. Aparecerá una lista de dispositivos de entrada debajo — marca tu micrófono. EchoWarp mezclará la transmisión y tu micrófono en la salida virtual. En Discord, selecciona el dispositivo virtual como tu micrófono.

**No tengo BlackHole / VB-Cable instalado.**
→ Ejecuta `EchoWarp doctor` — te dirá exactamente qué instalar y cómo. En Linux, no se necesita software adicional.

**El dispositivo loopback no aparece en la TUI.**
→ macOS: Instala BlackHole primero (`brew install blackhole-2ch`). Windows/Linux: los dispositivos loopback están integrados y deberían aparecer automáticamente.

**El dispositivo virtual muestra "adaptive" en lugar de una frecuencia de muestreo.**
→ Esto es normal. Los drivers de audio virtual (BlackHole, VB-Cable) se adaptan a la frecuencia de muestreo que use la aplicación — la tasa mostrada no tiene relevancia.

## Modo CLI

Todos los ajustes disponibles en la TUI también pueden pasarse como flags de CLI para scripting y automatización:

```bash
# Servidor
EchoWarp server -d 1 -P mypassword

# Cliente (descubre el servidor en LAN automáticamente si se omite -a)
EchoWarp client -a 192.168.1.10 -d 2 -P mypassword

# Listar dispositivos de audio
EchoWarp devices
```

### Modos

Consulta [Modos de Transmisión](#modos-de-transmisión) para descripciones detalladas de cada modo.

| Modo | Flag |
|------|------|
| Normal | *(por defecto)* |
| Inverso | `-r` |
| Dúplex | `-X` |
| Conferencia | `--conference` |

### Flags comunes

| Flag | Abreviatura | Descripción |
|------|-------------|-------------|
| `--device` | `-d` | ID del dispositivo de audio |
| `--device-name` | `-D` | Seleccionar dispositivo por nombre (coincidencia de subcadena) |
| `--password` | `-P` | Contraseña de autenticación |
| `--port` | `-p` | Puerto TCP (por defecto: 4415) |
| `--sample-rate` | | Frecuencia de muestreo (por defecto: 48000) |
| `--channels` | | 1=mono, 2=estéreo (por defecto: 1) |
| `--max-clients` | | Máx. clientes conectados (por defecto: 1) |
| `--virtual-mic` | | Crear dispositivo de micrófono virtual |
| `--config` | `-c` | Cargar archivo de configuración YAML |
| `--save-config` | `-s` | Guardar configuración actual en YAML |
| `--profile` | | Cargar perfil de dispositivo guardado |
| `--stun-server` | | Servidores STUN personalizados para traversal de NAT |
| `--tls-cert` / `--tls-key` | | Certificado y clave TLS |
| `--log-level` | | debug, info, warn, error |
| `--aec` | | Cancelación acústica de eco (dúplex) |
| `--loopback` | | Capturar audio del sistema (macOS, requiere BlackHole) |
| `--dry-run` | | Validar configuración y salir |

### Archivos de configuración

Prioridad de ajustes: flags CLI > variables de entorno (`ECHOWARP_*`) > archivo de configuración > valores por defecto.

La pantalla de configuración del TUI permite guardar y cargar archivos de configuración de forma interactiva. También puede usar flags de CLI:

```bash
# Guardar los ajustes actuales en un archivo
EchoWarp server -d 1 -P pass --channels 2 -s myconfig.yml

# Cargar ajustes desde un archivo
EchoWarp server -c myconfig.yml
```

## Instalación

### Binarios precompilados

Descarga desde la página de [Releases](https://github.com/lHumaNl/EchoWarp/releases). Disponible para:
- **macOS** — arm64 (Apple Silicon), amd64 (Intel), incluye paquete `.app`
- **Linux** — amd64, arm64, armv7, armv6
- **Windows** — amd64, arm64

### Compilar desde el código fuente

Requiere Go 1.22+ y las cabeceras de desarrollo de libopus.

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

El binario resultante enlaza opus estáticamente — no se necesita ninguna dependencia en tiempo de ejecución.

## Requisitos del sistema

- **macOS** 11.0+ (arm64, amd64)
- **Linux** (amd64, arm64, armv7, armv6)
- **Windows** 10+ (amd64, arm64)

Para el micrófono virtual:
- **macOS**: controlador de audio [BlackHole](https://github.com/ExistentialAudio/BlackHole)
- **Windows**: [VB-Audio Virtual Cable](https://vb-audio.com/Cable/)
- **Linux**: PulseAudio (`pactl`)

## Red y Firewall

### Servidor — puertos a abrir

| Puerto | Protocolo | Propósito |
|--------|-----------|-----------|
| `4415` | **TCP** | Señalización (autenticación + handshake WebRTC) |
| `4415` | **UDP** | Audio media (WebRTC). Multiplexado — un solo puerto gestiona todos los clientes |
| `4416` | **TCP** | Sonda de información de sesión (autoconfiguración del cliente) |

> El puerto `4415` es el predeterminado y se puede cambiar con `--port`. El puerto de información de sesión es siempre `port + 1`.

**En resumen:** abra **TCP 4415–4416** y **UDP 4415** de entrada en el servidor.

### Cliente — no se requieren puertos de entrada

El cliente solo realiza conexiones salientes. No se necesitan reglas de firewall ni reenvío de puertos en el lado del cliente.

### Descubrimiento en LAN

Si utiliza el descubrimiento automático por mDNS (`Ctrl+F` en la configuración), permita **UDP multicast en el puerto 5353**. Esto se puede desactivar con `--no-discovery`.

### Traversal de NAT (STUN / TURN)

EchoWarp utiliza servidores STUN para establecer conexiones a través de NAT. Si ambos lados están detrás de un NAT simétrico y no se puede establecer una conexión directa, se puede configurar un servidor de retransmisión TURN (`turn_servers` en la configuración). Tanto STUN como TURN son conexiones **salientes** y no requieren ninguna regla de firewall de entrada.

## Licencia

[MIT](../../LICENSE) — consulte [THIRD_PARTY_NOTICES.md](../../THIRD_PARTY_NOTICES.md) para las licencias de terceros.
