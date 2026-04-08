<p align="center">
  <img src="../../build/icons/icon_512.png" alt="EchoWarp Logo" width="128">
</p>

<h1 align="center">EchoWarp</h1>

<p align="center">
  Diffusion audio réseau en temps réel entre hôtes
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

Capturez l'audio sur une machine, lisez-le sur une autre — en temps réel via le réseau. EchoWarp utilise WebRTC pour le transport et Opus pour la compression, offrant un audio à faible latence avec chiffrement de bout en bout.

## Table des matières

- [Fonctionnalités](#fonctionnalités)
- [Démarrage rapide](#démarrage-rapide)
- [Interface TUI interactive](#interface-tui-interactive)
  - [Écran de configuration](#écran-de-configuration)
  - [Découverte LAN](#découverte-lan)
  - [Profils de périphériques](#profils-de-périphériques)
  - [Écran de diffusion](#écran-de-diffusion)
  - [Raccourcis clavier de l'interface TUI](#raccourcis-clavier-de-linterface-tui)
- [Modes de diffusion](#modes-de-diffusion)
  - [Normal — Sens unique : Serveur vers Clients](#normal-sens-unique-serveur-vers-clients)
  - [Inversé — Sens unique : Clients vers Serveur](#inversé-sens-unique-clients-vers-serveur)
  - [Duplex — Bidirectionnel](#duplex-bidirectionnel)
  - [Conférence — Mixage multi-utilisateurs (N:N)](#conférence-mixage-multi-utilisateurs-nn)
  - [Résumé des modes](#résumé-des-modes)
- [Guide de routage audio](#guide-de-routage-audio)
  - [Cas d'utilisation](#cas-dutilisation)
  - [Loopback — Capturer l'audio système](#loopback-capturer-laudio-système)
  - [Microphone virtuel — Router l'audio vers d'autres applications](#microphone-virtuel-router-laudio-vers-dautres-applications)
  - [Mixage du microphone local dans la sortie virtuelle](#mixage-du-microphone-local-dans-la-sortie-virtuelle)
  - [Sections de périphériques](#sections-de-périphériques)
  - [Diagnostics](#diagnostics)
  - [FAQ](#faq)
- [Mode CLI](#mode-cli)
  - [Modes](#modes)
  - [Indicateurs communs](#indicateurs-communs)
  - [Fichiers de configuration](#fichiers-de-configuration)
- [Installation](#installation)
  - [Binaires précompilés](#binaires-précompilés)
  - [Compilation depuis les sources](#compilation-depuis-les-sources)
- [Configuration requise](#configuration-requise)
- [Réseau & Pare-feu](#réseau-pare-feu)
  - [Serveur — ports à ouvrir](#serveur-ports-à-ouvrir)
  - [Client — aucun port entrant requis](#client-aucun-port-entrant-requis)
  - [Découverte LAN](#découverte-lan-1)
  - [Traversée de NAT (STUN / TURN)](#traversée-de-nat-stun-turn)
- [Licence](#licence)

## Fonctionnalités

- **Diffusion 1:1** — le serveur capture, le client lit (ou inversement)
- **Mode duplex** — audio bidirectionnel, les deux parties s'entendent mutuellement
- **Diffusion (1:N)** — un serveur diffuse vers plusieurs clients
- **Conférence (N:N)** — mixage multi-participants avec mixages personnels (total moins soi-même)
- **Microphone virtuel** — l'audio reçu apparaît comme une entrée micro (Discord, Zoom, OBS)
- **Découverte LAN** — détection automatique des serveurs via mDNS
- **Enregistrement** — enregistrement WAV : mixé, par piste, ou les deux
- **Accélération SIMD** — AVX/SSE (x86) et NEON (ARM) pour le mixage audio
- **Chiffrement de bout en bout** — AES + DTLS/SRTP
- **Multi-plateforme** — macOS, Linux, Windows

## Démarrage rapide

1. Téléchargez un binaire précompilé depuis la page [Releases](https://github.com/lHumaNl/EchoWarp/releases) (aucune dépendance — opus est lié statiquement)
2. Lancez `EchoWarp` — un menu interactif permet de choisir Serveur, Client, Diagnostics, etc. Ou passez directement à un mode avec `EchoWarp server` / `EchoWarp client`
3. Configurez tout dans l'interface TUI interactive et appuyez sur Entrée pour démarrer

> **Utilisateurs Windows :** L'archive de la release inclut `EchoWarp Server.bat` et `EchoWarp Client.bat` — double-cliquez sur celui dont vous avez besoin. Pas besoin d'ouvrir cmd.exe manuellement.

## Interface TUI interactive

L'interface TUI est l'interface principale d'EchoWarp. Exécutez simplement `EchoWarp server` ou `EchoWarp client` — l'écran de configuration interactif s'ouvre automatiquement.

### Écran de configuration

L'écran de configuration adopte une disposition en deux colonnes :

**Colonne gauche — Périphériques audio.** Liste tous les périphériques d'entrée/sortie disponibles avec leurs propriétés (canaux, fréquence d'échantillonnage, profondeur de bits). En mode duplex ou conférence, vous attribuez des rôles distincts de capture `[C]` et de lecture `[P]`. La création de périphérique virtuel est disponible directement depuis la liste.

**Colonne droite — Paramètres.** Tous les paramètres de session sont configurables ici avec validation en ligne :

| Paramètre | Description |
|-----------|-------------|
| Adresse du serveur | IP/nom d'hôte du serveur (client uniquement) |
| Port | Port TCP (par défaut : 4415) |
| Mot de passe | Mot de passe d'authentification |
| Mode | Normal / Inversé / Duplex |
| Clients max | Connexions maximales (serveur uniquement) |
| Fréquence d'échantillonnage | 8000 / 16000 / 24000 / 48000 Hz |
| Canaux | Mono / Stéréo |
| Débit Opus | 16–512 kbps |
| Annulation d'écho | AEC pour le mode duplex |
| TLS | Activer/désactiver avec les chemins du certificat et de la clé |
| Niveau de journalisation | debug / info / warn / error |

Naviguez entre les colonnes avec `Tab`, parcourez les champs avec les touches fléchées et appuyez sur `Enter` pour confirmer et démarrer la diffusion.

### Découverte LAN

Sur l'écran de configuration du client, appuyez sur `Ctrl+F` pour ouvrir le panneau de découverte. EchoWarp utilise mDNS pour trouver automatiquement les serveurs sur le réseau local — sélectionnez-en un dans la liste et les champs adresse/port sont remplis automatiquement.

### Profils de périphériques

Appuyez sur `Ctrl+O` pour ouvrir le panneau des profils. Enregistrez votre configuration de périphérique actuelle sous un profil nommé, chargez un profil précédemment sauvegardé ou supprimez ceux dont vous n'avez plus besoin. Les profils mémorisent les assignations et les rôles des périphériques.

### Écran de diffusion

Une fois connecté, l'interface TUI affiche l'état de la diffusion en temps réel :

- **Statistiques de connexion** — RTT, gigue, perte de paquets avec des barres de seuil indiquant la proximité des limites de qualité
- **Qualité de connexion** — indicateur global (Excellent / Bon / Acceptable / Mauvais) calculé à partir du RTT, de la gigue et des pertes
- **Analyseur de spectre** — visualisation FFT en temps réel du signal audio
- **Vumètres** — indicateurs de niveau audio par canal
- **Débit binaire** — affichage en direct du débit montant/descendant
- **Contrôles de périphérique** — réglage du volume, sourdine, changement de périphérique
- **Panneau de journaux** — vue des journaux défilante activable/désactivable

### Raccourcis clavier de l'interface TUI

| Touche | Action |
|--------|--------|
| `Tab` | Changer de colonne (configuration) |
| `Ctrl+F` | Découverte LAN (configuration client) |
| `Ctrl+O` | Profils de périphériques (configuration) |
| `Enter` | Confirmer et démarrer |
| `Ctrl+Q` | Quitter |
| `Ctrl+P` | Mettre en pause/reprendre la diffusion |
| `Ctrl+L` | Afficher/masquer le panneau de journaux |
| `Ctrl+M` | Activer/désactiver la sourdine |
| `+` / `-` | Augmenter/diminuer le volume |
| `Ctrl+Up/Down` | Faire défiler les journaux |
| `Ctrl+R` | Démarrer/arrêter l'enregistrement |
| `Ctrl+D` | Expulser un client (serveur) |
| `Ctrl+B` | Bannir un client (serveur) |

## Modes de diffusion

EchoWarp dispose de quatre modes de diffusion. Choisissez celui qui correspond à votre scénario — le mode est sélectionné dans l'écran de configuration de la TUI (côté serveur) ou détecté automatiquement (côté client).

### Normal — Sens unique : Serveur vers Clients

Le serveur capture l'audio et l'envoie à tous les clients connectés. Les clients ne font qu'écouter.

```
Server [captures audio] ───→ Client 1 [plays audio]
                         ├──→ Client 2 [plays audio]
                         └──→ Client N [plays audio]
```

**Quand l'utiliser :** diffuser de la musique/des podcasts à des auditeurs, diffuser l'audio système (YouTube, Spotify) vers une autre pièce, envoyer un flux loopback vers une machine distante.

**Périphériques nécessaires :**

| Côté | Capture (entrée) | Lecture (sortie) |
|------|:-:|:-:|
| Serveur | requis | — |
| Client | — | requis |

**Participants :** 1 serveur + 1…N clients (définissez *Clients max* dans la TUI).

---

### Inversé — Sens unique : Clients vers Serveur

L'opposé du mode Normal. Les clients capturent l'audio et l'envoient au serveur. Le serveur ne fait qu'écouter.

```
Client 1 [captures audio] ───→ Server [plays audio]
Client 2 [captures audio] ──┘
```

**Quand l'utiliser :** utiliser un microphone distant sur la machine serveur, collecter l'audio depuis un emplacement distant, envoyer le micro d'un client vers les haut-parleurs du serveur.

**Périphériques nécessaires :**

| Côté | Capture (entrée) | Lecture (sortie) |
|------|:-:|:-:|
| Serveur | — | requis |
| Client | requis | — |

**Participants :** 1 serveur + 1…N clients.

---

### Duplex — Bidirectionnel

Les deux côtés capturent et lisent simultanément. Tout le monde entend tout le monde.

```
Server [mic + speakers] ⟷ Client 1 [mic + speakers]
                        ⟷ Client 2 [mic + speakers]
```

**Quand l'utiliser :** appel vocal entre deux ou plusieurs machines, interphone entre pièces, session audio collaborative.

**Périphériques nécessaires :**

| Côté | Capture (entrée) | Lecture (sortie) |
|------|:-:|:-:|
| Serveur | requis | requis |
| Client | requis | requis |

**Participants :** 1 serveur + 1…N clients. Prend en charge l'AEC (annulation d'écho) pour éviter les boucles de retour.

---

### Conférence — Mixage multi-utilisateurs (N:N)

Le serveur agit comme un hub de mixage. Chaque participant (serveur + clients) envoie son audio et reçoit un mixage personnel de **tous les autres** (total moins soi-même). Cela empêche d'entendre sa propre voix.

```
Client 1 ──┐              ┌──→ Client 1 (hears Server + 2 + 3)
Client 2 ──┤  Server hub  ├──→ Client 2 (hears Server + 1 + 3)
Client 3 ──┘              └──→ Client 3 (hears Server + 1 + 2)
```

**Quand l'utiliser :** appels de groupe avec 3+ machines, répétitions à distance, conférences multi-participants.

**Périphériques nécessaires :**

| Côté | Capture (entrée) | Lecture (sortie) |
|------|:-:|:-:|
| Serveur (hub uniquement) | — | — |
| Serveur (participant) | requis | requis |
| Client | requis | requis |

Le serveur peut fonctionner en **hub uniquement** (pas d'audio local — juste le mixage pour les clients) ou en tant que participant complet avec son propre micro et ses haut-parleurs.

**Participants :** 1 serveur + 2…N clients. Prend en charge l'enregistrement (mixé, par piste, ou les deux), l'expulsion/le bannissement et le contrôle du volume par participant.

---

### Résumé des modes

| | Normal | Inversé | Duplex | Conférence |
|---|---|---|---|---|
| **Direction** | Serveur → Clients | Clients → Serveur | Les deux sens | Tout le monde ⟷ Tout le monde |
| **Périphériques serveur** | Capture uniquement | Lecture uniquement | Les deux | Les deux (ou aucun si hub) |
| **Périphériques client** | Lecture uniquement | Capture uniquement | Les deux | Les deux |
| **Participants max** | 1 + N | 1 + N | 1 + N | 1 + N |
| **Support AEC** | — | — | oui | oui |
| **Mixage personnel** | — | — | — | oui |
| **Enregistrement** | — | — | — | oui |
| **Indicateur CLI** | *(par défaut)* | `-r` | `-X` | `--conference` |

> **Astuce :** Le nombre de clients est contrôlé par le paramètre *Clients max* dans la TUI (par défaut : 1). Augmentez-le si vous avez besoin de scénarios de diffusion ou multi-utilisateurs.

## Guide de routage audio

### Cas d'utilisation

EchoWarp prend en charge plusieurs scénarios de routage audio. Le tableau ci-dessous indique quel mode et quelle configuration de périphériques utiliser pour chacun :

| Scénario | Mode | Périphériques serveur | Périphériques client |
|----------|------|-----------------------|----------------------|
| Diffuser le micro vers des haut-parleurs distants | Normal | Micro `[Input]` | Haut-parleurs `[Output]` |
| Diffuser l'audio système (YouTube, Spotify) | Normal | 🔄 Loopback `[Input]` | Haut-parleurs `[Output]` |
| Utiliser un micro distant dans Discord/Zoom | Reverse | Haut-parleurs `[Output]` | Micro `[Input]` |
| Chat vocal bidirectionnel | Duplex | Micro + Haut-parleurs | Micro + Haut-parleurs |
| Appel de groupe (3+ machines) | Conference | — (hub) | Micro + Haut-parleurs |
| Audio distant comme micro virtuel dans OBS | Normal | 🔄 Loopback `[Input]` | ⟡ Micro virtuel `[Output]` |
| Flux + micro local en un seul micro virtuel | Normal | Micro `[Input]` | ⟡ Virtuel `[Output]` + 🎤 Micro mixé |

### Loopback — Capturer l'audio système

Les périphériques loopback capturent tout l'audio joué sur une machine (musique, appels vidéo, sons de jeu) et le rendent disponible comme source d'entrée.

| Plateforme | Fonctionnement | Configuration |
|------------|----------------|---------------|
| **macOS** | Aggregate Device (haut-parleurs + BlackHole) | Installer [BlackHole](https://github.com/ExistentialAudio/BlackHole) : `brew install blackhole-2ch`. Les périphériques loopback apparaissent automatiquement dans la TUI |
| **Windows** | Loopback natif WASAPI | Intégré, aucun logiciel supplémentaire nécessaire |
| **Linux** | Sources monitor PulseAudio | Intégré avec PulseAudio/PipeWire |

Dans la TUI, les périphériques loopback sont marqués avec 🔄 et apparaissent dans la section Input.

### Microphone virtuel — Router l'audio vers d'autres applications

Un microphone virtuel fait apparaître l'audio reçu comme une entrée micro que Discord, Zoom, OBS et d'autres applications peuvent utiliser.

| Plateforme | Pilote | Installation |
|------------|--------|--------------|
| **macOS** | [BlackHole](https://github.com/ExistentialAudio/BlackHole) | `brew install blackhole-2ch` |
| **Windows** | [VB-Audio Virtual Cable](https://vb-audio.com/Cable/) | Télécharger depuis vb-audio.com |
| **Linux** | Null-sink PulseAudio | Créé depuis la TUI : Paramètres → Micro virtuel → Créer |

Sur macOS et Windows, installez le pilote et sélectionnez-le dans la section Output de la TUI. Sur Linux, la TUI crée automatiquement le sink virtuel — dans les autres applications, sélectionnez **"Monitor of EchoWarp"** comme microphone.

Les périphériques virtuels sont marqués avec ⟡ dans la TUI et affichent **adaptive** à la place d'un taux d'échantillonnage fixe.

### Mixage du microphone local dans la sortie virtuelle

Lorsque vous routez un flux distant vers un périphérique de sortie virtuel (BlackHole, VB-Cable), les applications tierces comme Discord ou Zoom ne voient que le flux — elles n'entendent pas votre microphone local. Si vous avez besoin que votre voix **et** le flux apparaissent comme une seule entrée micro, EchoWarp peut les mixer ensemble.

**Utilisation :** Dans la TUI, sélectionnez un périphérique virtuel dans la section Output. Les périphériques d'entrée apparaîtront en dessous sous forme de sous-éléments — cochez le microphone que vous souhaitez mixer :

```
🔊 Output
  ⟡ BlackHole 2ch                           stereo 32-bit adaptive   [✓]
      └─ 🎤 Built-in Microphone                                      [✓]
```

L'audio mixé (flux + micro) est écrit sur la sortie virtuelle. Dans Discord/Zoom/OBS, sélectionnez ce périphérique virtuel comme microphone — le flux et votre voix seront tous deux audibles.

**Alternatives au niveau du système d'exploitation** (sans utiliser le mixeur intégré) :

| Plateforme | Méthode | Détails |
|------------|---------|---------|
| **macOS** | Aggregate Device | Ouvrez Configuration audio et MIDI → Créez un Aggregate Device combinant votre micro + BlackHole. Sélectionnez l'agrégat comme entrée dans votre application |
| **Windows** | [VoiceMeeter](https://vb-audio.com/Voicemeeter/) | Mixeur virtuel gratuit — routez VB-Cable et votre micro dans VoiceMeeter, utilisez sa sortie comme micro dans votre application |
| **Linux** | PulseAudio combine-source | `pactl load-module module-combine-source source_name=combined slaves=<mic>,<virtual>.monitor` |

Le mixeur intégré fonctionne sur toutes les plateformes et ne nécessite aucune configuration supplémentaire.

### Sections de périphériques

La TUI n'affiche que les sections pertinentes au mode actuel :

| Mode + Côté | Sections visibles |
|-------------|-------------------|
| Serveur Normal / Client Reverse | 🎤 Input uniquement |
| Client Normal / Serveur Reverse | 🔊 Output uniquement |
| Duplex / Conference | 🎤 Input + 🔊 Output |

### Diagnostics

Exécutez `EchoWarp doctor` pour vérifier votre configuration audio :

```
Audio
[OK] Audio system: 2 input device(s), 2 output device(s)
[OK] Virtual audio: BlackHole 2ch detected
```

Si aucun pilote audio virtuel n'est trouvé, le doctor affichera les instructions d'installation pour votre plateforme.

### FAQ

**Je veux écouter de la musique de mon ordinateur sur des haut-parleurs dans une autre pièce.**
→ Exécutez `EchoWarp server` sur la machine musicale, sélectionnez le périphérique loopback 🔄 (vos haut-parleurs) dans Input. Exécutez `EchoWarp client` sur la machine distante, sélectionnez les haut-parleurs dans Output.

**Je veux utiliser un microphone distant pour Zoom/Discord.**
→ Exécutez `EchoWarp server` en mode Reverse sur la machine distante (celle avec le micro). Exécutez `EchoWarp client` sur votre machine, sélectionnez un périphérique audio virtuel (BlackHole/VB-Cable) dans Output. Dans Zoom/Discord, sélectionnez ce périphérique virtuel comme microphone.

**Je veux diffuser l'audio système (YouTube, Spotify) vers une autre machine.**
→ Identique à "musique dans une autre pièce" — utilisez le loopback 🔄 côté serveur. Le périphérique loopback capture tout ce qui joue via les haut-parleurs sélectionnés.

**Je veux que l'audio distant apparaisse comme une entrée micro dans OBS.**
→ Exécutez le serveur avec la source audio. Sur votre machine (client), sélectionnez un périphérique audio virtuel dans Output. Dans OBS, ajoutez une source "Audio Input Capture" et sélectionnez le périphérique virtuel.

**Je veux un chat vocal bidirectionnel entre deux ordinateurs.**
→ Utilisez le mode Duplex. Sur les deux machines, sélectionnez un microphone dans Input et des haut-parleurs dans Output.

**Je veux un appel de groupe avec 3+ machines.**
→ Utilisez le mode Conference. Le serveur agit comme un hub (aucun périphérique nécessaire). Chaque client sélectionne un micro et des haut-parleurs. Tout le monde entend tout le monde, sauf sa propre voix.

**J'utilise Moonlight/Sunshine (ou NVIDIA GameStream) et je veux que mon microphone fonctionne dans les jeux sur l'hôte.**
→ Lancez `EchoWarp server` en mode Reverse sur l'hôte de jeu (machine Sunshine/GameStream). Lancez `EchoWarp client` sur la machine Moonlight, sélectionnez votre microphone dans Input. Sur le serveur, sélectionnez un périphérique audio virtuel (BlackHole/VB-Cable) dans Output, ou activez `--virtual-mic` pour en créer un automatiquement. Dans votre jeu ou chat vocal sur l'hôte, sélectionnez ce périphérique virtuel comme microphone. Votre voix depuis le client Moonlight apparaîtra comme entrée micro sur l'hôte de jeu.

**Je veux que Discord entende à la fois le flux distant ET ma voix via un seul micro virtuel.**
→ Sur le client, sélectionnez un périphérique virtuel (BlackHole/VB-Cable) dans Output. Une liste de périphériques d'entrée apparaîtra en dessous — cochez votre microphone. EchoWarp mixera le flux et votre micro dans la sortie virtuelle. Dans Discord, sélectionnez le périphérique virtuel comme microphone.

**Je n'ai pas BlackHole / VB-Cable installé.**
→ Exécutez `EchoWarp doctor` — il vous indiquera exactement quoi installer et comment. Sur Linux, aucun logiciel supplémentaire n'est nécessaire.

**Le périphérique loopback n'apparaît pas dans la TUI.**
→ macOS : Installez d'abord BlackHole (`brew install blackhole-2ch`). Windows/Linux : les périphériques loopback sont intégrés et devraient apparaître automatiquement.

**Le périphérique virtuel affiche "adaptive" à la place d'un taux d'échantillonnage.**
→ C'est normal. Les pilotes audio virtuels (BlackHole, VB-Cable) s'adaptent au taux d'échantillonnage utilisé par l'application — le taux affiché n'est pas significatif.

<details>
<summary>macOS : « Impossible d'ouvrir EchoWarp » / avertissement Gatekeeper</summary>

macOS bloque les applications non signées. Pour autoriser l'exécution d'EchoWarp :

```bash
xattr -cr /path/to/EchoWarp       # pour le binaire
xattr -cr /path/to/EchoWarp.app   # pour le bundle .app
```

Sinon : **Réglages du système → Confidentialité et sécurité → « Ouvrir quand même »**

</details>

## Mode CLI

Tous les paramètres disponibles dans l'interface TUI peuvent également être passés en tant qu'indicateurs CLI pour les scripts et l'automatisation :

```bash
# Serveur
EchoWarp server -d 1 -P mypassword

# Client (découvre automatiquement le serveur sur le LAN si -a est omis)
EchoWarp client -a 192.168.1.10 -d 2 -P mypassword

# Lister les périphériques audio
EchoWarp devices
```

### Modes

Voir [Modes de diffusion](#modes-de-diffusion) pour les descriptions détaillées de chaque mode.

| Mode | Indicateur |
|------|------------|
| Normal | *(par défaut)* |
| Inversé | `-r` |
| Duplex | `-X` |
| Conférence | `--conference` |

### Indicateurs communs

| Indicateur | Court | Description |
|------------|-------|-------------|
| `--device` | `-d` | ID du périphérique audio |
| `--device-name` | `-D` | Sélectionner un périphérique par nom (correspondance de sous-chaîne) |
| `--password` | `-P` | Mot de passe d'authentification |
| `--port` | `-p` | Port TCP (par défaut : 4415) |
| `--sample-rate` | | Fréquence d'échantillonnage (par défaut : 48000) |
| `--channels` | | 1=mono, 2=stéréo (par défaut : 1) |
| `--max-clients` | | Nombre maximal de clients connectés (par défaut : 1) |
| `--virtual-mic` | | Créer un périphérique microphone virtuel |
| `--config` | `-c` | Charger un fichier de configuration YAML |
| `--save-config` | `-s` | Enregistrer les paramètres actuels en YAML |
| `--profile` | | Charger un profil de périphérique sauvegardé |
| `--stun-server` | | Serveurs STUN personnalisés pour la traversée NAT |
| `--tls-cert` / `--tls-key` | | Certificat et clé TLS |
| `--log-level` | | debug, info, warn, error |
| `--aec` | | Annulation acoustique d'écho (duplex) |
| `--loopback` | | Capturer l'audio système (macOS, nécessite BlackHole) |
| `--dry-run` | | Valider la configuration et quitter |

### Fichiers de configuration

Priorité des paramètres : indicateurs CLI > variables d'environnement (`ECHOWARP_*`) > fichier de configuration > valeurs par défaut.

L'écran de configuration du TUI permet de sauvegarder et charger des fichiers de configuration de manière interactive. Vous pouvez également utiliser les indicateurs CLI :

```bash
# Enregistrer les paramètres actuels dans un fichier
EchoWarp server -d 1 -P pass --channels 2 -s myconfig.yml

# Charger les paramètres depuis un fichier
EchoWarp server -c myconfig.yml
```

## Installation

### Binaires précompilés

Téléchargez depuis la page [Releases](https://github.com/lHumaNl/EchoWarp/releases). Disponible pour :
- **macOS** — arm64 (Apple Silicon), amd64 (Intel), inclut un bundle `.app`
- **Linux** — amd64, arm64, armv7, armv6
- **Windows** — amd64, arm64

### Compilation depuis les sources

Nécessite Go 1.22+ et les en-têtes de développement libopus.

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

Le binaire résultant lie opus statiquement — aucune dépendance d'exécution n'est nécessaire.

## Configuration requise

- **macOS** 11.0+ (arm64, amd64)
- **Linux** (amd64, arm64, armv7, armv6)
- **Windows** 10+ (amd64, arm64)

Pour le microphone virtuel :
- **macOS** : pilote audio [BlackHole](https://github.com/ExistentialAudio/BlackHole)
- **Windows** : [VB-Audio Virtual Cable](https://vb-audio.com/Cable/)
- **Linux** : PulseAudio (`pactl`)

## Réseau & Pare-feu

### Serveur — ports à ouvrir

| Port | Protocole | Utilisation |
|------|-----------|-------------|
| `4415` | **TCP** | Signalisation (authentification + handshake WebRTC) |
| `4415` | **UDP** | Flux audio (WebRTC). Multiplexé — un seul port gère tous les clients |
| `4416` | **TCP** | Sonde d'information de session (auto-configuration du client) |

> Le port `4415` est la valeur par défaut et peut être modifié avec `--port`. Le port d'information de session est toujours `port + 1`.

**En résumé :** ouvrez **TCP 4415–4416** et **UDP 4415** en entrée sur le serveur.

### Client — aucun port entrant requis

Le client n'effectue que des connexions sortantes. Aucune règle de pare-feu ou de redirection de port n'est nécessaire côté client.

### Découverte LAN

Si vous utilisez la découverte automatique mDNS (`Ctrl+F` dans la configuration), autorisez le **multicast UDP sur le port 5353**. Cette fonctionnalité peut être désactivée avec `--no-discovery`.

### Traversée de NAT (STUN / TURN)

EchoWarp utilise des serveurs STUN pour établir des connexions à travers le NAT. Si les deux côtés se trouvent derrière un NAT symétrique et qu'une connexion directe ne peut pas être établie, un serveur relais TURN peut être configuré (`turn_servers` dans la configuration). STUN et TURN sont des connexions **sortantes** et ne nécessitent aucune règle de pare-feu entrante.

## Licence

[MIT](../../LICENSE) — voir [THIRD_PARTY_NOTICES.md](../../THIRD_PARTY_NOTICES.md) pour les licences tierces.
