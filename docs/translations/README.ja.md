<p align="center">
  <img src="../../build/icons/icon_512.png" alt="EchoWarp Logo" width="128">
</p>

<h1 align="center">EchoWarp</h1>

<p align="center">
  ホスト間のリアルタイムネットワーク音声ストリーミング
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

1台のマシンで音声をキャプチャし、別のマシンでリアルタイムに再生 — ネットワーク越しに。EchoWarpはトランスポートにWebRTC、圧縮にOpusを使用し、エンドツーエンド暗号化による低遅延音声を実現します。

## 機能

- **1:1 ストリーミング** — サーバーがキャプチャし、クライアントが再生（または逆方向）
- **デュプレックスモード** — 双方向音声、双方が互いの音声を聞くことができる
- **ブロードキャスト (1:N)** — 1台のサーバーから複数のクライアントへストリーミング
- **カンファレンス (N:N)** — 個別ミックス（自分を除く全体）を伴うマルチ参加者ミキシング
- **バーチャルマイク** — 受信した音声をマイク入力として表示（Discord、Zoom、OBS）
- **LAN自動検出** — mDNSによるサーバーの自動検出
- **録音** — WAV録音：ミックス済み、トラックごと、またはその両方
- **SIMD高速化** — 音声ミキシングにAVX/SSE（x86）およびNEON（ARM）を使用
- **エンドツーエンド暗号化** — AES + DTLS/SRTP
- **クロスプラットフォーム** — macOS、Linux、Windows

## クイックスタート

1. [Releases](https://github.com/lHumaNl/EchoWarp/releases) からビルド済みバイナリをダウンロード（依存関係なし — opusは静的リンク）
2. `EchoWarp server` または `EchoWarp client` を実行
3. インタラクティブTUIですべての設定を行い、Enterを押してストリーミングを開始

> **Windowsユーザー：** リリースアーカイブには `EchoWarp Server.bat` と `EchoWarp Client.bat` が含まれています — 必要な方をダブルクリックしてください。cmd.exeを手動で開く必要はありません。

## インタラクティブTUI

TUIはEchoWarpの主要インターフェースです。`EchoWarp server` または `EchoWarp client` を実行するだけで、インタラクティブなセットアップ画面が自動的に開きます。

### セットアップ画面

セットアップ画面は2カラムレイアウトになっています：

**左カラム — オーディオデバイス。** 利用可能なすべての入出力デバイスとそのプロパティ（チャンネル数、サンプルレート、ビット深度）を一覧表示します。デュプレックスまたはカンファレンスモードでは、キャプチャ `[C]` と再生 `[P]` のロールを別々に割り当てます。バーチャルデバイスの作成はリストから直接行えます。

**右カラム — 設定。** インライン検証付きですべてのセッションパラメーターをここで設定できます：

| 設定 | 説明 |
|------|------|
| サーバーアドレス | サーバーのIPまたはホスト名（クライアントのみ） |
| ポート | TCPポート（デフォルト: 4415） |
| パスワード | 認証パスワード |
| モード | ノーマル / リバース / デュプレックス |
| 最大クライアント数 | 最大接続数（サーバーのみ） |
| サンプルレート | 8000 / 16000 / 24000 / 48000 Hz |
| チャンネル | モノラル / ステレオ |
| Opusビットレート | 16〜512 kbps |
| エコーキャンセレーション | デュプレックスモード用AEC |
| TLS | 証明書とキーパスを指定して有効/無効 |
| ログレベル | debug / info / warn / error |

`Tab` でカラム間を移動し、矢印キーでフィールド間を移動、`Enter` で確定してストリーミングを開始します。

### LAN自動検出

クライアントのセットアップ画面で `Ctrl+F` を押すと、検出オーバーレイが開きます。EchoWarpはmDNSを使用してローカルネットワーク上のサーバーを自動検出します — リストから選択するとアドレス/ポートフィールドが自動的に入力されます。

### デバイスプロファイル

`Ctrl+O` を押すとプロファイルオーバーレイが開きます。現在のデバイス設定を名前付きプロファイルとして保存したり、以前に保存したものを読み込んだり、不要なプロファイルを削除したりできます。プロファイルはデバイスの割り当てとロールを記憶します。

### ストリーミング画面

接続後、TUIはリアルタイムのストリーミング状態を表示します：

- **接続統計** — RTT、ジッター、パケットロスと品質限界への近接度を示すしきい値バー
- **接続品質** — RTT、ジッター、ロスから算出された総合インジケーター（Excellent / Good / Fair / Poor）
- **スペクトラムアナライザー** — 音声信号のリアルタイムFFT周波数可視化
- **VUメーター** — チャンネルごとの音声レベルメーター
- **ビットレート** — リアルタイムのアップロード/ダウンロードビットレート表示
- **デバイスコントロール** — 音量調整、ミュート、デバイス切り替え
- **ログパネル** — 表示/非表示を切り替え可能なスクロールログビュー

### TUIキーボードショートカット

| キー | 操作 |
|------|------|
| `Tab` | カラム切り替え（セットアップ） |
| `Ctrl+F` | LAN自動検出（クライアントセットアップ） |
| `Ctrl+O` | デバイスプロファイル（セットアップ） |
| `Enter` | 確定して開始 |
| `Ctrl+Q` | 終了 |
| `Ctrl+P` | ストリーミングの一時停止/再開 |
| `Ctrl+L` | ログパネルの表示/非表示 |
| `Ctrl+M` | ミュート/ミュート解除 |
| `+` / `-` | 音量アップ/ダウン |
| `Ctrl+Up/Down` | ログのスクロール |
| `Ctrl+R` | 録音の開始/停止 |
| `Ctrl+D` | クライアントのキック（サーバー） |
| `Ctrl+B` | クライアントのBAN（サーバー） |

## ストリーミングモード

EchoWarpには4つのストリーミングモードがあります。シナリオに合ったものを選択してください — モードはTUIセットアップ画面（サーバー側）で選択するか、自動検出（クライアント側）されます。

### ノーマル — 一方向：サーバーからクライアントへ

サーバーが音声をキャプチャし、接続されたすべてのクライアントに送信します。クライアントは聴くだけです。

```
Server [captures audio] ───→ Client 1 [plays audio]
                         ├──→ Client 2 [plays audio]
                         └──→ Client N [plays audio]
```

**使用シーン：** リスナーへの音楽/ポッドキャストのストリーミング、別の部屋へのシステムオーディオ（YouTube、Spotify）のブロードキャスト、リモートマシンへのループバックフィードの送信。

**必要なデバイス：**

| サイド | キャプチャ（入力） | 再生（出力） |
|--------|:-:|:-:|
| サーバー | 必須 | — |
| クライアント | — | 必須 |

**参加者数：** サーバー1台 + クライアント1〜N台（TUIで*最大クライアント数*を設定）。

---

### リバース — 一方向：クライアントからサーバーへ

ノーマルの逆です。クライアントが音声をキャプチャし、サーバーに送信します。サーバーは聴くだけです。

```
Client 1 [captures audio] ───→ Server [plays audio]
Client 2 [captures audio] ──┘
```

**使用シーン：** サーバーマシンでリモートマイクを使用する、リモートロケーションから音声を収集する、クライアントのマイクをサーバーのスピーカーに送信する。

**必要なデバイス：**

| サイド | キャプチャ（入力） | 再生（出力） |
|--------|:-:|:-:|
| サーバー | — | 必須 |
| クライアント | 必須 | — |

**参加者数：** サーバー1台 + クライアント1〜N台。

---

### デュプレックス — 双方向

双方が同時にキャプチャと再生を行います。全員が全員の音声を聞けます。

```
Server [mic + speakers] ⟷ Client 1 [mic + speakers]
                        ⟷ Client 2 [mic + speakers]
```

**使用シーン：** 2台以上のマシン間の音声通話、部屋間のインターコム、共同オーディオセッション。

**必要なデバイス：**

| サイド | キャプチャ（入力） | 再生（出力） |
|--------|:-:|:-:|
| サーバー | 必須 | 必須 |
| クライアント | 必須 | 必須 |

**参加者数：** サーバー1台 + クライアント1〜N台。フィードバックループを防止するAEC（エコーキャンセレーション）に対応。

---

### カンファレンス — マルチユーザーミキシング（N:N）

サーバーがミキシングハブとして機能します。各参加者（サーバー + クライアント）は自分の音声を送信し、**他の全員**のパーソナルミックス（全体から自分を引いたもの）を受信します。これにより自分の声が返ってくるのを防ぎます。

```
Client 1 ──┐              ┌──→ Client 1 (hears Server + 2 + 3)
Client 2 ──┤  Server hub  ├──→ Client 2 (hears Server + 1 + 3)
Client 3 ──┘              └──→ Client 3 (hears Server + 1 + 2)
```

**使用シーン：** 3台以上のマシンでのグループ通話、リモートリハーサル、マルチ参加者カンファレンス。

**必要なデバイス：**

| サイド | キャプチャ（入力） | 再生（出力） |
|--------|:-:|:-:|
| サーバー（ハブのみ） | — | — |
| サーバー（参加者） | 必須 | 必須 |
| クライアント | 必須 | 必須 |

サーバーは**ハブのみ**（ローカルオーディオなし — クライアント向けのミキシングのみ）として動作するか、自身のマイクとスピーカーを持つ完全な参加者として動作できます。

**参加者数：** サーバー1台 + クライアント2〜N台。録音（ミックス済み、トラックごと、またはその両方）、キック/BAN、参加者ごとの音量コントロールに対応。

---

### モード一覧

| | ノーマル | リバース | デュプレックス | カンファレンス |
|---|---|---|---|---|
| **方向** | サーバー → クライアント | クライアント → サーバー | 双方向 | 全員 ⟷ 全員 |
| **サーバーデバイス** | キャプチャのみ | 再生のみ | 両方 | 両方（ハブの場合はなし） |
| **クライアントデバイス** | 再生のみ | キャプチャのみ | 両方 | 両方 |
| **最大参加者数** | 1 + N | 1 + N | 1 + N | 1 + N |
| **AEC対応** | — | — | あり | あり |
| **パーソナルミックス** | — | — | — | あり |
| **録音** | — | — | — | あり |
| **CLIフラグ** | *（デフォルト）* | `-r` | `-X` | `--conference` |

> **ヒント：** クライアント数はTUIの*最大クライアント数*設定で制御します（デフォルト: 1）。ブロードキャストやマルチユーザーシナリオが必要な場合は、より大きな値を設定してください。

## オーディオルーティングガイド

### ユースケース

EchoWarpはいくつかのオーディオルーティングシナリオをサポートしています。以下の表は、各シナリオで使用するモードとデバイス設定を示しています：

| シナリオ | モード | サーバーデバイス | クライアントデバイス |
|----------|--------|-----------------|-------------------|
| マイクをリモートスピーカーにストリーミング | Normal | マイク `[Input]` | スピーカー `[Output]` |
| システムオーディオのストリーミング（YouTube、Spotify） | Normal | 🔄 ループバック `[Input]` | スピーカー `[Output]` |
| Discord/ZoomでリモートマイクをUse | Reverse | スピーカー `[Output]` | マイク `[Input]` |
| 双方向ボイスチャット | Duplex | マイク + スピーカー | マイク + スピーカー |
| グループ通話（3台以上） | Conference | —（ハブ） | マイク + スピーカー |
| OBSでのバーチャルマイクとしてリモートオーディオ | Normal | 🔄 ループバック `[Input]` | ⟡ バーチャルマイク `[Output]` |
| ストリーム + ローカルマイクを1つのバーチャルマイクとして | Normal | マイク `[Input]` | ⟡ バーチャル `[Output]` + 🎤 ミックスマイク |

### ループバック — システムオーディオのキャプチャ

ループバックデバイスは、マシン上で再生されているすべてのオーディオ（音楽、ビデオ通話、ゲームサウンド）をキャプチャし、入力ソースとして利用可能にします。

| プラットフォーム | 仕組み | セットアップ |
|----------------|--------|-------------|
| **macOS** | アグリゲートデバイス（スピーカー + BlackHole） | [BlackHole](https://github.com/ExistentialAudio/BlackHole)をインストール：`brew install blackhole-2ch`。ループバックデバイスはTUIに自動的に表示されます |
| **Windows** | WASAPIネイティブループバック | 組み込み済み、追加ソフトウェア不要 |
| **Linux** | PulseAudioモニターソース | PulseAudio/PipeWireに組み込み済み |

TUIでは、ループバックデバイスは🔄でマークされ、Inputセクションに表示されます。

### バーチャルマイクロフォン — 他のアプリへのオーディオルーティング

バーチャルマイクロフォンは、受信したオーディオをDiscord、Zoom、OBS、その他のアプリケーションが使用できるマイク入力として表示させます。

| プラットフォーム | ドライバー | インストール |
|----------------|-----------|-------------|
| **macOS** | [BlackHole](https://github.com/ExistentialAudio/BlackHole) | `brew install blackhole-2ch` |
| **Windows** | [VB-Audio Virtual Cable](https://vb-audio.com/Cable/) | vb-audio.comからダウンロード |
| **Linux** | PulseAudioヌルシンク | TUIから作成：Settings → Virtual mic → Create |

macOSとWindowsでは、ドライバーをインストールしてTUIのOutputセクションで選択します。Linuxでは、TUIが仮想シンクを自動的に作成します — 他のアプリでは**「Monitor of EchoWarp」**をマイクロフォンとして選択してください。

バーチャルデバイスはTUIで⟡でマークされ、固定サンプルレートの代わりに**adaptive**と表示されます。

### ローカルマイクのバーチャル出力へのミキシング

リモートストリームをバーチャル出力デバイス（BlackHole、VB-Cable）にルーティングすると、DiscordやZoomなどのサードパーティアプリにはストリームのみが聞こえます — ローカルマイクの音声は聞こえません。あなたの声**と**ストリームの両方を1つのマイク入力として表示させたい場合、EchoWarpはそれらをミックスできます。

**使い方：** TUIでOutputセクションのバーチャルデバイスを選択します。その下にサブアイテムとして入力デバイスが表示されます — ミックスしたいマイクにチェックを入れてください：

```
🔊 Output
  ⟡ BlackHole 2ch                           stereo 32-bit adaptive   [✓]
      └─ 🎤 Built-in Microphone                                      [✓]
```

ミックスされたオーディオ（ストリーム + マイク）がバーチャル出力に書き込まれます。Discord/Zoom/OBSで、そのバーチャルデバイスをマイクロフォンとして選択してください — ストリームとあなたの声の両方が聞こえるようになります。

**OSレベルの代替手段**（内蔵ミキサーを使用しない場合）：

| プラットフォーム | 方法 | 詳細 |
|----------------|------|------|
| **macOS** | アグリゲートデバイス | Audio MIDI設定を開き → マイク + BlackHoleを組み合わせたアグリゲートデバイスを作成。アプリでそのアグリゲートを入力として選択 |
| **Windows** | [VoiceMeeter](https://vb-audio.com/Voicemeeter/) | 無料の仮想ミキサー — VB-Cableとマイクの両方をVoiceMeeterにルーティングし、その出力をアプリでマイクとして使用 |
| **Linux** | PulseAudio combine-source | `pactl load-module module-combine-source source_name=combined slaves=<mic>,<virtual>.monitor` |

内蔵ミキサーはすべてのプラットフォームで動作し、追加の設定は不要です。

### デバイスセクション

TUIは現在のモードに関連するセクションのみを表示します：

| モード + サイド | 表示されるセクション |
|---------------|-------------------|
| Normalサーバー / Reverseクライアント | 🎤 Inputのみ |
| Normalクライアント / Reverseサーバー | 🔊 Outputのみ |
| Duplex / Conference | 🎤 Input + 🔊 Output |

### 診断

`EchoWarp doctor`を実行してオーディオ設定を確認します：

```
Audio
[OK] Audio system: 2 input device(s), 2 output device(s)
[OK] Virtual audio: BlackHole 2ch detected
```

バーチャルオーディオドライバーが見つからない場合、doctorはお使いのプラットフォーム向けのインストール手順を表示します。

### よくある質問

**コンピューターの音楽を別の部屋のスピーカーで聴きたい。**
→ 音楽マシンで`EchoWarp server`を実行し、Inputで🔄ループバックデバイス（お使いのスピーカー）を選択します。リモートマシンで`EchoWarp client`を実行し、Outputでスピーカーを選択します。

**Zoom/Discordでリモートマイクを使いたい。**
→ マイクのあるリモートマシンでReverseモードの`EchoWarp server`を実行します。自分のマシンで`EchoWarp client`を実行し、Outputでバーチャルオーディオデバイス（BlackHole/VB-Cable）を選択します。Zoom/Discordで、そのバーチャルデバイスをマイクロフォンとして選択します。

**システムオーディオ（YouTube、Spotify）を別のマシンにストリーミングしたい。**
→ 「別の部屋での音楽」と同じです — サーバー側で🔄ループバックを使用します。ループバックデバイスは選択したスピーカーで再生されているすべての音をキャプチャします。

**リモートオーディオをOBSのマイク入力として表示させたい。**
→ オーディオソースでサーバーを実行します。自分のマシン（クライアント）のOutputでバーチャルオーディオデバイスを選択します。OBSで「音声入力キャプチャ」ソースを追加し、バーチャルデバイスを選択します。

**2台のコンピューター間で双方向ボイスチャットをしたい。**
→ Duplexモードを使用します。両方のマシンで、InputにマイクロフォンをOutputにスピーカーを選択します。

**3台以上のマシンでグループ通話をしたい。**
→ Conferenceモードを使用します。サーバーはハブとして機能します（デバイス不要）。各クライアントはマイクとスピーカーを選択します。全員が互いの声を聞くことができます（自分の声は除く）。

**Discordでリモートストリームと自分の声の両方を1つのバーチャルマイクで聞かせたい。**
→ クライアントのOutputでバーチャルデバイス（BlackHole/VB-Cable）を選択します。その下に入力デバイスのリストが表示されます — マイクにチェックを入れてください。EchoWarpがストリームとマイクをバーチャル出力にミックスします。Discordで、そのバーチャルデバイスをマイクロフォンとして選択してください。

**BlackHole / VB-Cableがインストールされていない。**
→ `EchoWarp doctor`を実行してください — 何をどのようにインストールすべきか正確に教えてくれます。Linuxでは追加ソフトウェアは不要です。

**TUIにループバックデバイスが表示されない。**
→ macOS：まずBlackHoleをインストールしてください（`brew install blackhole-2ch`）。Windows/Linux：ループバックデバイスは組み込み済みで自動的に表示されるはずです。

**バーチャルデバイスにサンプルレートの代わりに「adaptive」と表示される。**
→ これは正常です。バーチャルオーディオドライバー（BlackHole、VB-Cable）はアプリケーションが使用するサンプルレートに適応します — 表示されるレートに意味はありません。

## CLIモード

TUIで利用可能なすべての設定は、スクリプトや自動化のためにCLIフラグとして渡すこともできます：

```bash
# サーバー
EchoWarp server -d 1 -P mypassword

# クライアント（-a を省略するとLAN上のサーバーを自動検出）
EchoWarp client -a 192.168.1.10 -d 2 -P mypassword

# オーディオデバイスの一覧表示
EchoWarp devices
```

### モード

各モードの詳細については[ストリーミングモード](#ストリーミングモード)を参照してください。

| モード | フラグ |
|--------|--------|
| ノーマル | *（デフォルト）* |
| リバース | `-r` |
| デュプレックス | `-X` |
| カンファレンス | `--conference` |

### 共通フラグ

| フラグ | 短縮形 | 説明 |
|--------|--------|------|
| `--device` | `-d` | オーディオデバイスID |
| `--device-name` | `-D` | デバイス名で選択（部分一致） |
| `--password` | `-P` | 認証パスワード |
| `--port` | `-p` | TCPポート（デフォルト: 4415） |
| `--sample-rate` | | サンプルレート（デフォルト: 48000） |
| `--channels` | | 1=モノラル、2=ステレオ（デフォルト: 1） |
| `--max-clients` | | 最大接続クライアント数（デフォルト: 1） |
| `--virtual-mic` | | バーチャルマイクデバイスの作成 |
| `--config` | `-c` | YAMLコンフィグファイルの読み込み |
| `--save-config` | `-s` | 現在の設定をYAMLに保存 |
| `--profile` | | 保存済みデバイスプロファイルの読み込み |
| `--stun-server` | | NAT越えのためのカスタムSTUNサーバー |
| `--tls-cert` / `--tls-key` | | TLS証明書とキー |
| `--log-level` | | debug, info, warn, error |
| `--aec` | | 音響エコーキャンセレーション（デュプレックス） |
| `--loopback` | | システム音声のキャプチャ（macOS、BlackHole必須） |
| `--dry-run` | | 設定を検証して終了 |

### 設定ファイル

設定の優先順位: CLIフラグ > 環境変数（`ECHOWARP_*`）> 設定ファイル > デフォルト値。

TUIセットアップ画面では、設定ファイルのインタラクティブな保存と読み込みが可能です。CLIフラグも使用できます：

```bash
# 現在の設定をファイルに保存
EchoWarp server -d 1 -P pass --channels 2 -s myconfig.yml

# ファイルから設定を読み込む
EchoWarp server -c myconfig.yml
```

## インストール

### ビルド済みバイナリ

[Releases](https://github.com/lHumaNl/EchoWarp/releases) ページからダウンロードできます。以下のプラットフォームに対応：
- **macOS** — arm64（Apple Silicon）、amd64（Intel）、`.app` バンドル付き
- **Linux** — amd64, arm64, armv7, armv6
- **Windows** — amd64, arm64

### ソースからビルド

Go 1.22以上とlibopusの開発ヘッダーが必要です。

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

生成されたバイナリはopusを静的リンクしているため、実行時の依存関係は不要です。

## システム要件

- **macOS** 11.0以上（arm64、amd64）
- **Linux**（amd64、arm64、armv7、armv6）
- **Windows** 10以上（amd64、arm64）

バーチャルマイク使用時：
- **macOS**: [BlackHole](https://github.com/ExistentialAudio/BlackHole) オーディオドライバー
- **Windows**: [VB-Audio Virtual Cable](https://vb-audio.com/Cable/)
- **Linux**: PulseAudio（`pactl`）

## ネットワークとファイアウォール

### サーバー側 — 開放が必要なポート

| ポート | プロトコル | 用途 |
|------|----------|---------|
| `4415` | **TCP** | シグナリング（認証 + WebRTC ハンドシェイク） |
| `4415` | **UDP** | 音声メディア（WebRTC）。多重化されており、1つのポートで全クライアントを処理 |
| `4416` | **TCP** | セッション情報プローブ（クライアント自動設定） |

> ポート `4415` はデフォルト値であり、`--port` で変更できます。セッション情報ポートは常に `port + 1` となります。

**要約:** サーバー側のインバウンドで **TCP 4415–4416** および **UDP 4415** を開放してください。

### クライアント側 — インバウンドポートは不要

クライアントはアウトバウンド接続のみを行います。クライアント側ではファイアウォールやポートフォワーディングの設定は不要です。

### LAN ディスカバリー

mDNS 自動検出（セットアップ画面で `Ctrl+F`）を使用する場合は、**UDP マルチキャスト（ポート 5353）** を許可してください。`--no-discovery` で無効にできます。

### NAT トラバーサル（STUN / TURN）

EchoWarp は STUN サーバーを使用して NAT 越しの接続を確立します。双方が対称 NAT の背後にあり直接接続が確立できない場合は、TURN リレーサーバーを設定できます（設定ファイルの `turn_servers`）。STUN と TURN はどちらも**アウトバウンド**接続であり、インバウンドのファイアウォールルールは必要ありません。

## ライセンス

[MIT](../../LICENSE) — サードパーティライセンスについては [THIRD_PARTY_NOTICES.md](../../THIRD_PARTY_NOTICES.md) を参照してください。
