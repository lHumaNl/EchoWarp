<p align="center">
  <img src="../../build/icons/icon_512.png" alt="EchoWarp Logo" width="128">
</p>

<h1 align="center">EchoWarp</h1>

<p align="center">
  Streaming de áudio em rede em tempo real entre hosts
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

Capture áudio em uma máquina e reproduza em outra — em tempo real pela rede. O EchoWarp usa WebRTC para transporte e Opus para compressão, proporcionando áudio de baixa latência com criptografia de ponta a ponta.

## Funcionalidades

- **Streaming 1:1** — o servidor captura, o cliente reproduz (ou o inverso)
- **Modo duplex** — áudio bidirecional, ambos os lados se ouvem
- **Broadcast (1:N)** — um servidor transmite para múltiplos clientes
- **Conferência (N:N)** — mixagem com vários participantes e mixagens individuais (total menos o próprio)
- **Microfone virtual** — o áudio recebido aparece como entrada de microfone (Discord, Zoom, OBS)
- **Descoberta em rede local** — detecção automática de servidores via mDNS
- **Gravação** — gravação WAV: mixada, por faixa, ou ambas
- **Aceleração SIMD** — AVX/SSE (x86) e NEON (ARM) para mixagem de áudio
- **Criptografia de ponta a ponta** — AES + DTLS/SRTP
- **Multiplataforma** — macOS, Linux, Windows

## Início Rápido

1. Baixe um binário pré-compilado em [Releases](https://github.com/lHumaNl/EchoWarp/releases) (sem dependências — o opus é vinculado estaticamente)
2. Execute `EchoWarp server` ou `EchoWarp client`
3. Configure tudo na TUI interativa e pressione Enter para iniciar

> **Usuários Windows:** O arquivo de release inclui `EchoWarp Server.bat` e `EchoWarp Client.bat` — basta dar duplo clique no que você precisa. Não é necessário abrir o cmd.exe manualmente.

## TUI Interativa

A TUI é a interface principal do EchoWarp. Basta executar `EchoWarp server` ou `EchoWarp client` — a tela de configuração interativa abre automaticamente.

### Tela de Configuração

A tela de configuração possui um layout de duas colunas:

**Coluna esquerda — Dispositivos de áudio.** Lista todos os dispositivos de entrada/saída disponíveis com suas propriedades (canais, taxa de amostragem, profundidade de bits). No modo duplex ou conferência, você atribui papéis separados de captura `[C]` e reprodução `[P]`. A criação de dispositivos virtuais está disponível diretamente na lista.

**Coluna direita — Configurações.** Todos os parâmetros da sessão são configuráveis aqui com validação inline:

| Configuração | Descrição |
|---------|-------------|
| Endereço do servidor | IP/hostname do servidor (somente cliente) |
| Porta | Porta TCP (padrão: 4415) |
| Senha | Senha de autenticação |
| Modo | Normal / Reverso / Duplex |
| Máx. de clientes | Máximo de conexões (somente servidor) |
| Taxa de amostragem | 8000 / 16000 / 24000 / 48000 Hz |
| Canais | Mono / Estéreo |
| Bitrate Opus | 16–512 kbps |
| Cancelamento de eco | AEC para modo duplex |
| TLS | Ativar/desativar com caminhos do certificado e chave |
| Nível de log | debug / info / warn / error |

Navegue entre as colunas com `Tab`, mova-se pelos campos com as teclas de seta e pressione `Enter` para confirmar e iniciar o streaming.

### Descoberta em Rede Local

Na tela de configuração do cliente, pressione `Ctrl+F` para abrir a sobreposição de descoberta. O EchoWarp usa mDNS para encontrar automaticamente servidores na rede local — selecione um da lista e os campos de endereço/porta são preenchidos automaticamente.

### Perfis de Dispositivo

Pressione `Ctrl+O` para abrir a sobreposição de perfis. Salve sua configuração atual de dispositivos como um perfil nomeado, carregue um salvo anteriormente ou exclua perfis que não são mais necessários. Os perfis lembram atribuições e papéis de dispositivos.

### Tela de Streaming

Após conectado, a TUI exibe o status do streaming em tempo real:

- **Estatísticas de conexão** — RTT, jitter, perda de pacotes com barras de limite mostrando a proximidade dos limites de qualidade
- **Qualidade da conexão** — indicador geral (Excelente / Bom / Regular / Ruim) calculado a partir de RTT, jitter e perda
- **Analisador de espectro** — visualização FFT de frequência em tempo real do sinal de áudio
- **Medidores VU** — medidores de nível de áudio por canal
- **Bitrate** — exibição ao vivo do bitrate de upload/download
- **Controles de dispositivo** — ajuste de volume, mudo, troca de dispositivo
- **Painel de log** — visualização de log com rolagem ativável/desativável

### Atalhos de Teclado da TUI

| Tecla | Ação |
|-----|--------|
| `Tab` | Alternar colunas (configuração) |
| `Ctrl+F` | Descoberta em rede local (configuração do cliente) |
| `Ctrl+O` | Perfis de dispositivo (configuração) |
| `Enter` | Confirmar e iniciar |
| `Ctrl+Q` | Sair |
| `Ctrl+P` | Pausar/retomar streaming |
| `Ctrl+L` | Ativar/desativar painel de log |
| `Ctrl+M` | Mutar/desmutar |
| `+` / `-` | Aumentar/diminuir volume |
| `Ctrl+Up/Down` | Rolar logs |
| `Ctrl+R` | Iniciar/parar gravação |
| `Ctrl+D` | Expulsar cliente (servidor) |
| `Ctrl+B` | Banir cliente (servidor) |

## Modos de Streaming

O EchoWarp possui quatro modos de streaming. Escolha o que melhor se adapta ao seu cenário — o modo é selecionado na tela de configuração da TUI (lado servidor) ou detectado automaticamente (lado cliente).

### Normal — Unidirecional: Servidor para Clientes

O servidor captura o áudio e o envia para todos os clientes conectados. Os clientes apenas escutam.

```
Server [captura áudio] ───→ Client 1 [reproduz áudio]
                        ├──→ Client 2 [reproduz áudio]
                        └──→ Client N [reproduz áudio]
```

**Quando usar:** transmitir música/podcasts para ouvintes, transmitir áudio do sistema (YouTube, Spotify) para outro ambiente, enviar um feed de loopback para uma máquina remota.

**Dispositivos necessários:**

| Lado | Captura (entrada) | Reprodução (saída) |
|------|:-:|:-:|
| Servidor | necessário | — |
| Cliente | — | necessário |

**Participantes:** 1 servidor + 1…N clientes (defina *Máx. clientes* na TUI).

---

### Reverso — Unidirecional: Clientes para Servidor

O oposto do Normal. Os clientes capturam o áudio e o enviam para o servidor. O servidor apenas escuta.

```
Client 1 [captura áudio] ───→ Server [reproduz áudio]
Client 2 [captura áudio] ──┘
```

**Quando usar:** usar um microfone remoto na máquina do servidor, coletar áudio de um local remoto, enviar o microfone de um cliente para os alto-falantes do servidor.

**Dispositivos necessários:**

| Lado | Captura (entrada) | Reprodução (saída) |
|------|:-:|:-:|
| Servidor | — | necessário |
| Cliente | necessário | — |

**Participantes:** 1 servidor + 1…N clientes.

---

### Duplex — Bidirecional

Ambos os lados capturam e reproduzem simultaneamente. Todos ouvem todos.

```
Server [mic + alto-falantes] ⟷ Client 1 [mic + alto-falantes]
                             ⟷ Client 2 [mic + alto-falantes]
```

**Quando usar:** chamada de voz entre duas ou mais máquinas, intercomunicador entre ambientes, sessão de áudio colaborativa.

**Dispositivos necessários:**

| Lado | Captura (entrada) | Reprodução (saída) |
|------|:-:|:-:|
| Servidor | necessário | necessário |
| Cliente | necessário | necessário |

**Participantes:** 1 servidor + 1…N clientes. Suporta AEC (cancelamento de eco) para evitar loops de realimentação.

---

### Conferência — Mixagem Multi-Usuário (N:N)

O servidor atua como hub de mixagem. Cada participante (servidor + clientes) envia seu áudio e recebe uma mixagem pessoal de **todos os outros** (total menos si mesmo). Isso evita ouvir sua própria voz.

```
Client 1 ──┐              ┌──→ Client 1 (ouve Server + 2 + 3)
Client 2 ──┤  Server hub  ├──→ Client 2 (ouve Server + 1 + 3)
Client 3 ──┘              └──→ Client 3 (ouve Server + 1 + 2)
```

**Quando usar:** chamadas em grupo com 3+ máquinas, ensaios remotos, conferências com múltiplos participantes.

**Dispositivos necessários:**

| Lado | Captura (entrada) | Reprodução (saída) |
|------|:-:|:-:|
| Servidor (somente hub) | — | — |
| Servidor (participante) | necessário | necessário |
| Cliente | necessário | necessário |

O servidor pode funcionar como **somente hub** (sem áudio local — apenas mixagem para os clientes) ou como participante completo com seu próprio microfone e alto-falantes.

**Participantes:** 1 servidor + 2…N clientes. Suporta gravação (mixada, por faixa ou ambas), expulsão/banimento e controle de volume por participante.

---

### Resumo dos Modos

| | Normal | Reverso | Duplex | Conferência |
|---|---|---|---|---|
| **Direção** | Servidor → Clientes | Clientes → Servidor | Ambas as direções | Todos ⟷ Todos |
| **Dispositivos do servidor** | Somente captura | Somente reprodução | Ambos | Ambos (ou nenhum se hub) |
| **Dispositivos do cliente** | Somente reprodução | Somente captura | Ambos | Ambos |
| **Máx. participantes** | 1 + N | 1 + N | 1 + N | 1 + N |
| **Suporte a AEC** | — | — | sim | sim |
| **Mixagem pessoal** | — | — | — | sim |
| **Gravação** | — | — | — | sim |
| **Flag CLI** | *(padrão)* | `-r` | `-X` | `--conference` |

> **Dica:** O número de clientes é controlado pela configuração *Máx. clientes* na TUI (padrão: 1). Aumente se precisar de cenários de transmissão ou multi-usuário.

## Guia de Roteamento de Áudio

### Casos de Uso

O EchoWarp suporta vários cenários de roteamento de áudio. A tabela abaixo mostra qual modo e configuração de dispositivos usar em cada caso:

| Cenário | Modo | Dispositivos do servidor | Dispositivos do cliente |
|---------|------|--------------------------|-------------------------|
| Transmitir microfone para alto-falantes remotos | Normal | Microfone `[Input]` | Alto-falantes `[Output]` |
| Transmitir áudio do sistema (YouTube, Spotify) | Normal | 🔄 Loopback `[Input]` | Alto-falantes `[Output]` |
| Usar microfone remoto no Discord/Zoom | Reverse | Alto-falantes `[Output]` | Microfone `[Input]` |
| Bate-papo de voz bidirecional | Duplex | Mic + Alto-falantes | Mic + Alto-falantes |
| Chamada em grupo (3+ máquinas) | Conference | — (hub) | Mic + Alto-falantes |
| Áudio remoto como microfone virtual no OBS | Normal | 🔄 Loopback `[Input]` | ⟡ Microfone virtual `[Output]` |
| Stream + microfone local como um microfone virtual | Normal | Microfone `[Input]` | ⟡ Virtual `[Output]` + 🎤 Mix mic |

### Loopback — Capturar Áudio do Sistema

Os dispositivos loopback capturam todo o áudio sendo reproduzido em uma máquina (música, chamadas de vídeo, som de jogos) e o disponibilizam como fonte de entrada.

| Plataforma | Como funciona | Configuração |
|------------|--------------|--------------|
| **macOS** | Aggregate Device (alto-falantes + BlackHole) | Instale o [BlackHole](https://github.com/ExistentialAudio/BlackHole): `brew install blackhole-2ch`. Os dispositivos loopback aparecem automaticamente na TUI |
| **Windows** | Loopback nativo WASAPI | Integrado, sem software adicional necessário |
| **Linux** | Fontes monitor do PulseAudio | Integrado com PulseAudio/PipeWire |

Na TUI, os dispositivos loopback são marcados com 🔄 e aparecem na seção Input.

### Microfone Virtual — Encaminhar Áudio para Outros Aplicativos

Um microfone virtual faz o áudio recebido aparecer como entrada de microfone que Discord, Zoom, OBS e outros aplicativos podem usar.

| Plataforma | Driver | Instalação |
|------------|--------|------------|
| **macOS** | [BlackHole](https://github.com/ExistentialAudio/BlackHole) | `brew install blackhole-2ch` |
| **Windows** | [VB-Audio Virtual Cable](https://vb-audio.com/Cable/) | Baixe em vb-audio.com |
| **Linux** | PulseAudio null-sink | Criado pela TUI: Settings → Virtual mic → Create |

No macOS e Windows, instale o driver e selecione-o na seção Output da TUI. No Linux, a TUI cria o sink virtual automaticamente — em outros aplicativos, selecione **"Monitor of EchoWarp"** como microfone.

Os dispositivos virtuais são marcados com ⟡ na TUI e exibem **adaptive** em vez de uma taxa de amostragem fixa.

### Mixagem do Microfone Local na Saída Virtual

Quando você encaminha um stream remoto para um dispositivo de saída virtual (BlackHole, VB-Cable), aplicativos como Discord ou Zoom enxergam apenas o stream — eles não ouvem o seu microfone local. Se você precisa que tanto a sua voz **quanto** o stream apareçam como uma única entrada de microfone, o EchoWarp pode mixá-los.

**Como usar:** Na TUI, selecione um dispositivo virtual na seção Output. Os dispositivos de entrada aparecerão abaixo dele como sub-itens — marque o microfone que deseja mixar:

```
🔊 Output
  ⟡ BlackHole 2ch                           stereo 32-bit adaptive   [✓]
      └─ 🎤 Built-in Microphone                                      [✓]
```

O áudio mixado (stream + microfone) é gravado na saída virtual. No Discord/Zoom/OBS, selecione esse dispositivo virtual como microfone — tanto o stream quanto a sua voz serão ouvidos.

**Alternativas no nível do SO** (sem usar o mixer integrado):

| Plataforma | Como | Detalhes |
|------------|------|----------|
| **macOS** | Aggregate Device | Abra o Audio MIDI Setup → Crie um Aggregate Device combinando seu microfone + BlackHole. Selecione o aggregate como entrada no seu aplicativo |
| **Windows** | [VoiceMeeter](https://vb-audio.com/Voicemeeter/) | Mixer virtual gratuito — encaminhe tanto o VB-Cable quanto o seu microfone para o VoiceMeeter, use a saída dele como microfone no seu aplicativo |
| **Linux** | PulseAudio combine-source | `pactl load-module module-combine-source source_name=combined slaves=<mic>,<virtual>.monitor` |

O mixer integrado funciona em todas as plataformas e não requer configuração adicional.

### Seções de Dispositivos

A TUI exibe apenas as seções relevantes para o modo atual:

| Modo + Lado | Seções visíveis |
|-------------|----------------|
| Servidor Normal / Cliente Reverse | 🎤 Somente Input |
| Cliente Normal / Servidor Reverse | 🔊 Somente Output |
| Duplex / Conference | 🎤 Input + 🔊 Output |

### Diagnóstico

Execute `EchoWarp doctor` para verificar sua configuração de áudio:

```
Audio
[OK] Audio system: 2 input device(s), 2 output device(s)
[OK] Virtual audio: BlackHole 2ch detected
```

Se nenhum driver de áudio virtual for encontrado, o doctor exibirá instruções de instalação para a sua plataforma.

### Perguntas Frequentes

**Quero ouvir música do meu computador em alto-falantes em outro cômodo.**
→ Execute `EchoWarp server` na máquina com a música, selecione o dispositivo 🔄 loopback (seus alto-falantes) em Input. Execute `EchoWarp client` na máquina remota, selecione os alto-falantes em Output.

**Quero usar um microfone remoto no Zoom/Discord.**
→ Execute `EchoWarp server` no modo Reverse na máquina remota (a que tem o microfone). Execute `EchoWarp client` na sua máquina, selecione um dispositivo de áudio virtual (BlackHole/VB-Cable) em Output. No Zoom/Discord, selecione esse dispositivo virtual como microfone.

**Quero transmitir áudio do sistema (YouTube, Spotify) para outra máquina.**
→ Igual a "música em outro cômodo" — use 🔄 loopback no lado do servidor. O dispositivo loopback captura tudo o que está sendo reproduzido pelos alto-falantes selecionados.

**Quero que o áudio remoto apareça como entrada de microfone no OBS.**
→ Execute o servidor com a fonte de áudio. Na sua máquina (cliente), selecione um dispositivo de áudio virtual em Output. No OBS, adicione uma fonte "Audio Input Capture" e selecione o dispositivo virtual.

**Quero um bate-papo de voz bidirecional entre dois computadores.**
→ Use o modo Duplex. Em ambas as máquinas, selecione um microfone em Input e alto-falantes em Output.

**Quero uma chamada em grupo com 3 ou mais máquinas.**
→ Use o modo Conference. O servidor atua como hub (sem necessidade de dispositivos). Cada cliente seleciona um microfone e alto-falantes. Todos ouvem todos os outros, exceto a própria voz.

**Quero que o Discord ouça tanto o stream remoto QUANTO a minha voz através de um microfone virtual.**
→ No cliente, selecione um dispositivo virtual (BlackHole/VB-Cable) em Output. Uma lista de dispositivos de entrada aparecerá abaixo — marque o seu microfone. O EchoWarp irá mixar o stream e o seu microfone na saída virtual. No Discord, selecione o dispositivo virtual como microfone.

**Não tenho o BlackHole / VB-Cable instalado.**
→ Execute `EchoWarp doctor` — ele informará exatamente o que instalar e como. No Linux, nenhum software adicional é necessário.

**O dispositivo loopback não aparece na TUI.**
→ macOS: Instale o BlackHole primeiro (`brew install blackhole-2ch`). Windows/Linux: os dispositivos loopback são integrados e devem aparecer automaticamente.

**O dispositivo virtual exibe "adaptive" em vez de uma taxa de amostragem.**
→ Isso é normal. Os drivers de áudio virtual (BlackHole, VB-Cable) se adaptam à taxa de amostragem usada pelo aplicativo — a taxa exibida não tem significado prático.

## Modo CLI

Todas as configurações disponíveis na TUI também podem ser passadas como flags de CLI para scripts e automação:

```bash
# Servidor
EchoWarp server -d 1 -P mypassword

# Cliente (descobre automaticamente o servidor na rede local se -a for omitido)
EchoWarp client -a 192.168.1.10 -d 2 -P mypassword

# Listar dispositivos de áudio
EchoWarp devices
```

### Modos

Consulte [Modos de Streaming](#modos-de-streaming) para descrições detalhadas de cada modo.

| Modo | Flag |
|------|------|
| Normal | *(padrão)* |
| Reverso | `-r` |
| Duplex | `-X` |
| Conferência | `--conference` |

### Flags Comuns

| Flag | Abreviação | Descrição |
|------|-------|-------------|
| `--device` | `-d` | ID do dispositivo de áudio |
| `--device-name` | `-D` | Selecionar dispositivo por nome (correspondência de substring) |
| `--password` | `-P` | Senha de autenticação |
| `--port` | `-p` | Porta TCP (padrão: 4415) |
| `--sample-rate` | | Taxa de amostragem (padrão: 48000) |
| `--channels` | | 1=mono, 2=estéreo (padrão: 1) |
| `--max-clients` | | Máximo de clientes conectados (padrão: 1) |
| `--virtual-mic` | | Criar dispositivo de microfone virtual |
| `--config` | `-c` | Carregar arquivo de configuração YAML |
| `--save-config` | `-s` | Salvar configurações atuais em YAML |
| `--profile` | | Carregar perfil de dispositivo salvo |
| `--stun-server` | | Servidores STUN personalizados para travessia de NAT |
| `--tls-cert` / `--tls-key` | | Certificado e chave TLS |
| `--log-level` | | debug, info, warn, error |
| `--aec` | | Cancelamento acústico de eco (duplex) |
| `--loopback` | | Capturar áudio do sistema (macOS, requer BlackHole) |
| `--dry-run` | | Validar configuração e sair |

### Arquivos de Configuração

Prioridade das configurações: flags de CLI > variáveis de ambiente (`ECHOWARP_*`) > arquivo de configuração > padrões.

A tela de configuração do TUI permite salvar e carregar arquivos de configuração de forma interativa. Você também pode usar flags de CLI:

```bash
# Salvar as configurações atuais em um arquivo
EchoWarp server -d 1 -P pass --channels 2 -s myconfig.yml

# Carregar configurações de um arquivo
EchoWarp server -c myconfig.yml
```

## Instalação

### Binários pré-compilados

Baixe na página de [Releases](https://github.com/lHumaNl/EchoWarp/releases). Disponível para:
- **macOS** — arm64 (Apple Silicon), amd64 (Intel), inclui pacote `.app`
- **Linux** — amd64, arm64, armv7, armv6
- **Windows** — amd64, arm64

### Compilar a partir do código-fonte

Requer Go 1.22+ e cabeçalhos de desenvolvimento do libopus.

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

O binário resultante vincula o opus estaticamente — nenhuma dependência em tempo de execução é necessária.

## Requisitos do Sistema

- **macOS** 11.0+ (arm64, amd64)
- **Linux** (amd64, arm64, armv7, armv6)
- **Windows** 10+ (amd64, arm64)

Para microfone virtual:
- **macOS**: driver de áudio [BlackHole](https://github.com/ExistentialAudio/BlackHole)
- **Linux**: PulseAudio (`pactl`)

## Rede & Firewall

### Servidor — portas a abrir

| Porta | Protocolo | Finalidade |
|-------|-----------|------------|
| `4415` | **TCP** | Sinalização (autenticação + handshake WebRTC) |
| `4415` | **UDP** | Mídia de áudio (WebRTC). Multiplexada — uma porta atende todos os clientes |
| `4416` | **TCP** | Sonda de informações da sessão (autoconfiguração do cliente) |

> A porta `4415` é o padrão e pode ser alterada com `--port`. A porta de informações da sessão é sempre `port + 1`.

**Resumindo:** abra **TCP 4415–4416** e **UDP 4415** de entrada no servidor.

### Cliente — nenhuma porta de entrada necessária

O cliente apenas faz conexões de saída. Nenhuma regra de firewall ou encaminhamento de porta é necessária no lado do cliente.

### Descoberta na rede local

Se você usar a descoberta automática via mDNS (`Ctrl+F` na configuração), permita **multicast UDP na porta 5353**. Isso pode ser desativado com `--no-discovery`.

### Travessia de NAT (STUN / TURN)

O EchoWarp usa servidores STUN para estabelecer conexões através de NAT. Se ambos os lados estiverem atrás de NAT simétrico e uma conexão direta não puder ser estabelecida, um servidor de retransmissão TURN pode ser configurado (`turn_servers` no config). Tanto STUN quanto TURN são conexões **de saída** e não exigem nenhuma regra de firewall de entrada.

## Licença

[MIT](../../LICENSE) — consulte [THIRD_PARTY_NOTICES.md](../../THIRD_PARTY_NOTICES.md) para licenças de terceiros.
