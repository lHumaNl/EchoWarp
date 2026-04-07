<p align="center">
  <img src="../../build/icons/icon_512.png" alt="EchoWarp Logo" width="128">
</p>

<h1 align="center">EchoWarp</h1>

<p align="center">
  Ağ üzerinden gerçek zamanlı ses akışı
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

Bir makinede sesi yakalayın, başka bir makinede ağ üzerinden gerçek zamanlı olarak oynatın. EchoWarp, taşıma için WebRTC ve sıkıştırma için Opus kullanarak uçtan uca şifreleme ile düşük gecikmeli ses iletimi sağlar.

## Özellikler

- **1:1 akış** — sunucu yakalar, istemci oynatır (ya da tersi)
- **Çift yönlü mod** — iki yönlü ses, her iki taraf birbirini duyar
- **Yayın (1:N)** — bir sunucu birden fazla istemciye akış yapar
- **Konferans (N:N)** — kişisel karışımlarla çok katılımcılı ses karıştırma (toplam eksi kendisi)
- **Sanal mikrofon** — alınan ses bir mikrofon girişi olarak görünür (Discord, Zoom, OBS)
- **LAN keşfi** — mDNS aracılığıyla otomatik sunucu tespiti
- **Kayıt** — WAV kaydı: karışık, parça başına veya her ikisi
- **SIMD hızlandırma** — ses karıştırma için AVX/SSE (x86) ve NEON (ARM)
- **Uçtan uca şifreleme** — AES + DTLS/SRTP
- **Çapraz platform** — macOS, Linux, Windows

## Hızlı Başlangıç

1. [Releases](https://github.com/lHumaNl/EchoWarp/releases) sayfasından önceden derlenmiş bir ikili dosya indirin (bağımlılık yok — opus statik olarak bağlıdır)
2. `EchoWarp` çalıştırın — etkileşimli menüden Sunucu, İstemci, Tanılama ve daha fazlasını seçin. Ya da doğrudan bir moda geçin: `EchoWarp server` / `EchoWarp client`
3. Her şeyi etkileşimli TUI'de yapılandırın ve başlatmak için Enter tuşuna basın

> **Windows kullanıcıları:** Yayın arşivi `EchoWarp Server.bat` ve `EchoWarp Client.bat` dosyalarını içerir — ihtiyacınız olana çift tıklayın. cmd.exe'yi manuel olarak açmanıza gerek yoktur.

## Etkileşimli TUI

TUI, EchoWarp'ın birincil arayüzüdür. Sadece `EchoWarp server` veya `EchoWarp client` komutunu çalıştırın — etkileşimli kurulum ekranı otomatik olarak açılır.

### Kurulum Ekranı

Kurulum ekranı iki sütunlu bir düzene sahiptir:

**Sol sütun — Ses cihazları.** Tüm mevcut giriş/çıkış cihazlarını özellikleriyle birlikte listeler (kanal sayısı, örnekleme hızı, bit derinliği). Çift yönlü veya konferans modunda ayrı yakalama `[C]` ve oynatma `[P]` rolleri atayabilirsiniz. Sanal cihaz oluşturma doğrudan listeden yapılabilir.

**Sağ sütun — Ayarlar.** Tüm oturum parametreleri satır içi doğrulama ile burada yapılandırılabilir:

| Ayar | Açıklama |
|------|----------|
| Sunucu adresi | Sunucu IP/ana makine adı (yalnızca istemci) |
| Port | TCP portu (varsayılan: 4415) |
| Parola | Kimlik doğrulama parolası |
| Mod | Normal / Ters / Çift Yönlü |
| Maksimum istemci | Maksimum bağlantı sayısı (yalnızca sunucu) |
| Örnekleme hızı | 8000 / 16000 / 24000 / 48000 Hz |
| Kanallar | Mono / Stereo |
| Opus bit hızı | 16–512 kbps |
| Eko iptali | Çift yönlü mod için AEC |
| TLS | Sertifika ve anahtar yollarıyla etkinleştir/devre dışı bırak |
| Log seviyesi | debug / info / warn / error |

Sütunlar arasında `Tab` ile gezinin, alanlar arasında ok tuşlarıyla hareket edin ve akışı onaylamak ve başlatmak için `Enter` tuşuna basın.

### LAN Keşfi

İstemci kurulum ekranında, keşif katmanını açmak için `Ctrl+F` tuşuna basın. EchoWarp, yerel ağdaki sunucuları otomatik olarak bulmak için mDNS kullanır — listeden birini seçin ve adres/port alanları otomatik olarak doldurulur.

### Cihaz Profilleri

Profil katmanını açmak için `Ctrl+O` tuşuna basın. Mevcut cihaz yapılandırmanızı adlandırılmış bir profil olarak kaydedin, daha önce kaydedilmiş bir profil yükleyin veya artık ihtiyaç duymadığınız profilleri silin. Profiller cihaz atamalarını ve rollerini hatırlar.

### Akış Ekranı

Bağlantı kurulduğunda, TUI gerçek zamanlı akış durumunu gösterir:

- **Bağlantı istatistikleri** — RTT, jitter, paket kaybı; kalite limitlerinin yakınlığını gösteren eşik çubukları ile birlikte
- **Bağlantı kalitesi** — RTT, jitter ve kayıptan hesaplanan genel gösterge (Mükemmel / İyi / Orta / Zayıf)
- **Spektrum analizörü** — ses sinyalinin gerçek zamanlı FFT frekans görselleştirmesi
- **VU metreler** — kanal başına ses seviyesi göstergeleri
- **Bit hızı** — canlı yükleme/indirme bit hızı gösterimi
- **Cihaz kontrolleri** — ses seviyesi ayarı, susturma, cihaz değiştirme
- **Log paneli** — açılıp kapanabilen kaydırılabilir log görünümü

### TUI Klavye Kısayolları

| Tuş | İşlem |
|-----|-------|
| `Tab` | Sütunları değiştir (kurulum) |
| `Ctrl+F` | LAN keşfi (istemci kurulumu) |
| `Ctrl+O` | Cihaz profilleri (kurulum) |
| `Enter` | Onayla ve başlat |
| `Ctrl+Q` | Çıkış |
| `Ctrl+P` | Akışı duraklat/devam ettir |
| `Ctrl+L` | Log panelini aç/kapat |
| `Ctrl+M` | Sesi aç/kapat |
| `+` / `-` | Ses seviyesini artır/azalt |
| `Ctrl+Up/Down` | Loglarda kaydır |
| `Ctrl+R` | Kaydı başlat/durdur |
| `Ctrl+D` | İstemciyi at (sunucu) |
| `Ctrl+B` | İstemciyi yasakla (sunucu) |

## Akış Modları

EchoWarp'ın dört akış modu vardır. Senaryonuza uygun olanı seçin — mod TUI kurulum ekranında (sunucu tarafında) seçilir veya otomatik olarak algılanır (istemci tarafında).

### Normal — Tek Yön: Sunucudan İstemcilere

Sunucu sesi yakalar ve bağlı tüm istemcilere gönderir. İstemciler yalnızca dinler.

```
Server [sesi yakalar] ───→ Client 1 [sesi oynatır]
                      ├──→ Client 2 [sesi oynatır]
                      └──→ Client N [sesi oynatır]
```

**Ne zaman kullanılır:** dinleyicilere müzik/podcast akışı, sistem sesini (YouTube, Spotify) başka bir odaya yayınlama, uzak bir makineye loopback beslemesi gönderme.

**Gereken cihazlar:**

| Taraf | Yakalama (giriş) | Oynatma (çıkış) |
|-------|:-:|:-:|
| Sunucu | gerekli | — |
| İstemci | — | gerekli |

**Katılımcılar:** 1 sunucu + 1…N istemci (TUI'de *Maksimum istemci* ayarını yapın).

---

### Ters — Tek Yön: İstemcilerden Sunucuya

Normal modun tersi. İstemciler sesi yakalar ve sunucuya gönderir. Sunucu yalnızca dinler.

```
Client 1 [sesi yakalar] ───→ Server [sesi oynatır]
Client 2 [sesi yakalar] ──┘
```

**Ne zaman kullanılır:** sunucu makinesinde uzak bir mikrofon kullanma, uzak bir konumdan ses toplama, istemcinin mikrofonunu sunucu hoparlörlerine gönderme.

**Gereken cihazlar:**

| Taraf | Yakalama (giriş) | Oynatma (çıkış) |
|-------|:-:|:-:|
| Sunucu | — | gerekli |
| İstemci | gerekli | — |

**Katılımcılar:** 1 sunucu + 1…N istemci.

---

### Çift Yönlü — İki Yönlü

Her iki taraf aynı anda yakalar ve oynatır. Herkes herkesi duyar.

```
Server [mikrofon + hoparlörler] ⟷ Client 1 [mikrofon + hoparlörler]
                                ⟷ Client 2 [mikrofon + hoparlörler]
```

**Ne zaman kullanılır:** iki veya daha fazla makine arasında sesli arama, odalar arası interkom, işbirlikçi ses oturumu.

**Gereken cihazlar:**

| Taraf | Yakalama (giriş) | Oynatma (çıkış) |
|-------|:-:|:-:|
| Sunucu | gerekli | gerekli |
| İstemci | gerekli | gerekli |

**Katılımcılar:** 1 sunucu + 1…N istemci. Geri besleme döngülerini önlemek için AEC (eko iptali) destekler.

---

### Konferans — Çok Kullanıcılı Karıştırma (N:N)

Sunucu bir karıştırma merkezi olarak çalışır. Her katılımcı (sunucu + istemciler) sesini gönderir ve **diğer herkesin** kişisel karışımını alır (toplam eksi kendisi). Bu, kendi sesinizi duymanızı önler.

```
Client 1 ──┐              ┌──→ Client 1 (Server + 2 + 3'ü duyar)
Client 2 ──┤  Server hub  ├──→ Client 2 (Server + 1 + 3'ü duyar)
Client 3 ──┘              └──→ Client 3 (Server + 1 + 2'yi duyar)
```

**Ne zaman kullanılır:** 3+ makineyle grup aramaları, uzaktan provalar, çok katılımcılı konferanslar.

**Gereken cihazlar:**

| Taraf | Yakalama (giriş) | Oynatma (çıkış) |
|-------|:-:|:-:|
| Sunucu (yalnızca hub) | — | — |
| Sunucu (katılımcı) | gerekli | gerekli |
| İstemci | gerekli | gerekli |

Sunucu **yalnızca hub** olarak (yerel ses yok — yalnızca istemciler için karıştırma) veya kendi mikrofonu ve hoparlörleriyle tam katılımcı olarak çalışabilir.

**Katılımcılar:** 1 sunucu + 2…N istemci. Kayıt (karışık, parça başına veya her ikisi), atma/yasaklama ve katılımcı başına ses seviyesi kontrolü destekler.

---

### Mod Özeti

| | Normal | Ters | Çift Yönlü | Konferans |
|---|---|---|---|---|
| **Yön** | Sunucu → İstemciler | İstemciler → Sunucu | Her iki yön | Herkes ⟷ Herkes |
| **Sunucu cihazları** | Yalnızca yakalama | Yalnızca oynatma | Her ikisi | Her ikisi (veya hub ise hiçbiri) |
| **İstemci cihazları** | Yalnızca oynatma | Yalnızca yakalama | Her ikisi | Her ikisi |
| **Maks katılımcı** | 1 + N | 1 + N | 1 + N | 1 + N |
| **AEC desteği** | — | — | evet | evet |
| **Kişisel karışım** | — | — | — | evet |
| **Kayıt** | — | — | — | evet |
| **CLI bayrağı** | *(varsayılan)* | `-r` | `-X` | `--conference` |

> **İpucu:** İstemci sayısı TUI'deki *Maksimum istemci* ayarıyla kontrol edilir (varsayılan: 1). Yayın veya çok kullanıcılı senaryolar için daha yüksek bir değer ayarlayın.

## Ses Yönlendirme Kılavuzu

### Kullanım Senaryoları

EchoWarp birkaç ses yönlendirme senaryosunu destekler. Aşağıdaki tablo, her senaryo için hangi mod ve cihaz kurulumunun kullanılacağını gösterir:

| Senaryo | Mod | Sunucu cihazları | İstemci cihazları |
|---------|-----|------------------|-------------------|
| Mikrofonu uzak hoparlörlere aktar | Normal | Mikrofon `[Input]` | Hoparlörler `[Output]` |
| Sistem sesini aktar (YouTube, Spotify) | Normal | 🔄 Loopback `[Input]` | Hoparlörler `[Output]` |
| Uzak mikrofonu Discord/Zoom'da kullan | Reverse | Hoparlörler `[Output]` | Mikrofon `[Input]` |
| Çift yönlü sesli sohbet | Duplex | Mikrofon + Hoparlörler | Mikrofon + Hoparlörler |
| Grup araması (3+ makine) | Conference | — (hub) | Mikrofon + Hoparlörler |
| Uzak sesi OBS'de sanal mikrofon olarak kullan | Normal | 🔄 Loopback `[Input]` | ⟡ Sanal mikrofon `[Output]` |
| Akış + yerel mikrofon tek sanal mikrofon olarak | Normal | Mikrofon `[Input]` | ⟡ Sanal `[Output]` + 🎤 Mix mikrofon |

### Loopback — Sistem Sesini Yakala

Loopback cihazları, bir makinede çalan tüm sesi (müzik, görüntülü aramalar, oyun sesi) yakalar ve bunu bir giriş kaynağı olarak kullanılabilir hale getirir.

| Platform | Nasıl çalışır | Kurulum |
|----------|--------------|---------|
| **macOS** | Aggregate Device (hoparlörler + BlackHole) | [BlackHole](https://github.com/ExistentialAudio/BlackHole) yükleyin: `brew install blackhole-2ch`. Loopback cihazları TUI'de otomatik olarak görünür |
| **Windows** | WASAPI yerel loopback | Yerleşik, ek yazılım gerekmez |
| **Linux** | PulseAudio monitor kaynakları | PulseAudio/PipeWire ile yerleşik |

TUI'de loopback cihazları 🔄 ile işaretlenir ve Giriş bölümünde görünür.

### Sanal Mikrofon — Sesi Diğer Uygulamalara Yönlendir

Sanal mikrofon, alınan sesin Discord, Zoom, OBS ve diğer uygulamaların kullanabileceği bir mikrofon girişi olarak görünmesini sağlar.

| Platform | Sürücü | Kurulum |
|----------|--------|---------|
| **macOS** | [BlackHole](https://github.com/ExistentialAudio/BlackHole) | `brew install blackhole-2ch` |
| **Windows** | [VB-Audio Virtual Cable](https://vb-audio.com/Cable/) | vb-audio.com adresinden indirin |
| **Linux** | PulseAudio null-sink | TUI'den oluşturulur: Ayarlar → Sanal mikrofon → Oluştur |

macOS ve Windows'ta sürücüyü yükleyin ve TUI'nin Çıkış bölümünden seçin. Linux'ta TUI sanal sink'i otomatik olarak oluşturur — diğer uygulamalarda mikrofon olarak **"Monitor of EchoWarp"** seçeneğini belirleyin.

Sanal cihazlar TUI'de ⟡ ile işaretlenir ve sabit bir örnek hız yerine **adaptive** gösterir.

### Yerel Mikrofonu Sanal Çıkışa Karıştırma

Uzak bir akışı sanal çıkış cihazına (BlackHole, VB-Cable) yönlendirdiğinizde, Discord veya Zoom gibi üçüncü taraf uygulamalar yalnızca akışı görür — yerel mikrofonunuzu duymazlar. Hem sesinizin **hem de** akışın tek bir mikrofon girişi olarak görünmesini istiyorsanız, EchoWarp bunları birlikte karıştırabilir.

**Nasıl kullanılır:** TUI'de Çıkış bölümünden bir sanal cihaz seçin. Altında giriş cihazları alt öğeler olarak görünecektir — karıştırmak istediğiniz mikrofonu işaretleyin:

```
🔊 Çıkış
  ⟡ BlackHole 2ch                           stereo 32-bit adaptive   [✓]
      └─ 🎤 Dahili Mikrofon                                          [✓]
```

Karıştırılmış ses (akış + mikrofon) sanal çıkışa yazılır. Discord/Zoom/OBS'de o sanal cihazı mikrofon olarak seçin — hem akış hem de sesiniz duyulacaktır.

**İşletim sistemi düzeyinde alternatifler** (yerleşik karıştırıcı kullanmadan):

| Platform | Nasıl | Ayrıntılar |
|----------|-------|------------|
| **macOS** | Aggregate Device | Ses MIDI Kurulumu'nu açın → Mikrofonunuz + BlackHole'u birleştiren Aggregate Device oluşturun. Uygulamanızda birleşik cihazı giriş olarak seçin |
| **Windows** | [VoiceMeeter](https://vb-audio.com/Voicemeeter/) | Ücretsiz sanal karıştırıcı — hem VB-Cable'ı hem de mikrofonunuzu VoiceMeeter'a yönlendirin, çıkışını uygulamanızda mikrofon olarak kullanın |
| **Linux** | PulseAudio combine-source | `pactl load-module module-combine-source source_name=combined slaves=<mic>,<virtual>.monitor` |

Yerleşik karıştırıcı tüm platformlarda çalışır ve ek yapılandırma gerektirmez.

### Cihaz Bölümleri

TUI yalnızca mevcut moda uygun bölümleri gösterir:

| Mod + Taraf | Görünür bölümler |
|-------------|-----------------|
| Normal sunucu / Reverse istemci | 🎤 Yalnızca Giriş |
| Normal istemci / Reverse sunucu | 🔊 Yalnızca Çıkış |
| Duplex / Conference | 🎤 Giriş + 🔊 Çıkış |

### Tanılama

Ses kurulumunuzu kontrol etmek için `EchoWarp doctor` çalıştırın:

```
Audio
[OK] Audio system: 2 input device(s), 2 output device(s)
[OK] Virtual audio: BlackHole 2ch detected
```

Sanal ses sürücüsü bulunamazsa, doctor platformunuz için kurulum talimatlarını gösterecektir.

### SSS

**Bilgisayarımdaki müziği başka bir odadaki hoparlörlerden dinlemek istiyorum.**
→ Müzik makinesinde `EchoWarp server` çalıştırın, Giriş'te 🔄 loopback cihazını (hoparlörlerinizi) seçin. Uzak makinede `EchoWarp client` çalıştırın, Çıkış'ta hoparlörleri seçin.

**Zoom/Discord için uzak bir mikrofon kullanmak istiyorum.**
→ Uzak makinede (mikrofonun bulunduğu makine) Reverse modunda `EchoWarp server` çalıştırın. Kendi makinenizde `EchoWarp client` çalıştırın, Çıkış'ta sanal bir ses cihazı (BlackHole/VB-Cable) seçin. Zoom/Discord'da o sanal cihazı mikrofon olarak seçin.

**Sistem sesini (YouTube, Spotify) başka bir makineye aktarmak istiyorum.**
→ "Başka bir odada müzik" ile aynıdır — sunucu tarafında 🔄 loopback kullanın. Loopback cihazı, seçilen hoparlörlerden çalan her şeyi yakalar.

**Uzak sesin OBS'de mikrofon girişi olarak görünmesini istiyorum.**
→ Ses kaynağıyla sunucuyu çalıştırın. Kendi makinenizde (istemci), Çıkış'ta sanal bir ses cihazı seçin. OBS'de "Audio Input Capture" kaynağı ekleyin ve sanal cihazı seçin.

**İki bilgisayar arasında çift yönlü sesli sohbet yapmak istiyorum.**
→ Duplex modunu kullanın. Her iki makinede de Giriş'te mikrofon, Çıkış'ta hoparlör seçin.

**3+ makineyle grup araması yapmak istiyorum.**
→ Conference modunu kullanın. Sunucu hub görevi görür (cihaz gerekmez). Her istemci bir mikrofon ve hoparlör seçer. Herkes kendi sesini duymadan diğerlerini duyar.

**Moonlight/Sunshine (veya NVIDIA GameStream) kullanıyorum ve mikrofonumun ana bilgisayardaki oyunlarda çalışmasını istiyorum.**
→ Oyun ana bilgisayarında (Sunshine/GameStream makinesi) `EchoWarp server`'ı Reverse modunda çalıştırın. Moonlight makinesinde `EchoWarp client`'ı çalıştırın, Input'ta mikrofonunuzu seçin. Sunucuda, Output'ta bir sanal ses cihazı (BlackHole/VB-Cable) seçin veya otomatik oluşturmak için `--virtual-mic`'i etkinleştirin. Ana bilgisayardaki oyununuzda veya sesli sohbetinizde bu sanal cihazı mikrofon olarak seçin. Moonlight istemcisinden gelen sesiniz, oyun ana bilgisayarında mikrofon girişi olarak görünecektir.

**Discord'un hem uzak akışı HEM DE sesimi tek bir sanal mikrofon üzerinden duymasını istiyorum.**
→ İstemcide Çıkış'ta bir sanal cihaz (BlackHole/VB-Cable) seçin. Altında giriş cihazları listesi görünecektir — mikrofonunuzu işaretleyin. EchoWarp akışı ve mikrofonunuzu sanal çıkışa karıştıracaktır. Discord'da sanal cihazı mikrofon olarak seçin.

**BlackHole / VB-Cable kurulu değil.**
→ `EchoWarp doctor` çalıştırın — tam olarak neyi ve nasıl yükleyeceğinizi söyleyecektir. Linux'ta ek yazılım gerekmez.

**Loopback cihazı TUI'de görünmüyor.**
→ macOS: Önce BlackHole'u yükleyin (`brew install blackhole-2ch`). Windows/Linux: Loopback cihazları yerleşiktir ve otomatik olarak görünmelidir.

**Sanal cihaz örnek hız yerine "adaptive" gösteriyor.**
→ Bu normaldir. Sanal ses sürücüleri (BlackHole, VB-Cable) uygulamanın kullandığı örnek hıza uyum sağlar — görüntülenen hızın bir önemi yoktur.

<details>
<summary>macOS: "EchoWarp açılamıyor" / Gatekeeper uyarısı</summary>

macOS imzalanmamış uygulamaları engeller. EchoWarp'ın çalışmasına izin vermek için:

```bash
xattr -cr /path/to/EchoWarp       # ikili dosya için
xattr -cr /path/to/EchoWarp.app   # .app paketi için
```

Alternatif olarak: **Sistem Ayarları → Gizlilik ve Güvenlik → "Yine de İzin Ver"**

</details>

## CLI Modu

TUI'de mevcut olan tüm ayarlar, komut dosyası oluşturma ve otomasyon için CLI bayrakları olarak da geçirilebilir:

```bash
# Sunucu
EchoWarp server -d 1 -P mypassword

# İstemci (-a atlanırsa LAN'da sunucuyu otomatik olarak keşfeder)
EchoWarp client -a 192.168.1.10 -d 2 -P mypassword

# Ses cihazlarını listele
EchoWarp devices
```

### Modlar

Her modun ayrıntılı açıklaması için [Akış Modları](#akış-modları) bölümüne bakın.

| Mod | Bayrak |
|-----|--------|
| Normal | *(varsayılan)* |
| Ters | `-r` |
| Çift Yönlü | `-X` |
| Konferans | `--conference` |

### Yaygın Bayraklar

| Bayrak | Kısa | Açıklama |
|--------|------|----------|
| `--device` | `-d` | Ses cihazı ID'si |
| `--device-name` | `-D` | Cihazı ada göre seç (alt dizi eşleşmesi) |
| `--password` | `-P` | Kimlik doğrulama parolası |
| `--port` | `-p` | TCP portu (varsayılan: 4415) |
| `--sample-rate` | | Örnekleme hızı (varsayılan: 48000) |
| `--channels` | | 1=mono, 2=stereo (varsayılan: 1) |
| `--max-clients` | | Maksimum bağlı istemci (varsayılan: 1) |
| `--virtual-mic` | | Sanal mikrofon cihazı oluştur |
| `--config` | `-c` | YAML yapılandırma dosyası yükle |
| `--save-config` | `-s` | Mevcut ayarları YAML'a kaydet |
| `--profile` | | Kaydedilmiş cihaz profili yükle |
| `--stun-server` | | NAT geçişi için özel STUN sunucuları |
| `--tls-cert` / `--tls-key` | | TLS sertifikası ve anahtarı |
| `--log-level` | | debug, info, warn, error |
| `--aec` | | Akustik eko iptali (çift yönlü) |
| `--loopback` | | Sistem sesini yakala (macOS, BlackHole gerektirir) |
| `--dry-run` | | Yapılandırmayı doğrula ve çık |

### Yapılandırma Dosyaları

Ayar önceliği: CLI bayrakları > ortam değişkenleri (`ECHOWARP_*`) > yapılandırma dosyası > varsayılanlar.

TUI kurulum ekranı, yapılandırma dosyalarını etkileşimli olarak kaydetmenize ve yüklemenize olanak tanır. CLI bayraklarını da kullanabilirsiniz:

```bash
# Mevcut ayarları bir dosyaya kaydet
EchoWarp server -d 1 -P pass --channels 2 -s myconfig.yml

# Bir dosyadan ayarları yükle
EchoWarp server -c myconfig.yml
```

## Kurulum

### Önceden Derlenmiş İkililer

[Releases](https://github.com/lHumaNl/EchoWarp/releases) sayfasından indirin. Şunlar için mevcuttur:
- **macOS** — arm64 (Apple Silicon), amd64 (Intel), `.app` paketi dahil
- **Linux** — amd64, arm64, armv7, armv6
- **Windows** — amd64, arm64

### Kaynak Koddan Derleme

Go 1.22+ ve libopus geliştirme başlıkları gerektirir.

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

Oluşturulan ikili dosya, opus'u statik olarak bağlar — çalışma zamanında bağımlılık gerekmez.

## Sistem Gereksinimleri

- **macOS** 11.0+ (arm64, amd64)
- **Linux** (amd64, arm64, armv7, armv6)
- **Windows** 10+ (amd64, arm64)

Sanal mikrofon için:
- **macOS**: [BlackHole](https://github.com/ExistentialAudio/BlackHole) ses sürücüsü
- **Windows**: [VB-Audio Virtual Cable](https://vb-audio.com/Cable/)
- **Linux**: PulseAudio (`pactl`)

## Ağ & Güvenlik Duvarı

### Sunucu — açılması gereken portlar

| Port | Protokol | Amaç |
|------|----------|------|
| `4415` | **TCP** | Sinyal (kimlik doğrulama + WebRTC el sıkışması) |
| `4415` | **UDP** | Ses medyası (WebRTC). Çoğullanmış — tek port tüm istemcilere hizmet verir |
| `4416` | **TCP** | Oturum bilgisi sorgulama (istemci otomatik yapılandırması) |

> `4415` portu varsayılandır ve `--port` ile değiştirilebilir. Oturum bilgisi portu her zaman `port + 1`'dir.

**Kısaca:** sunucuda gelen bağlantılar için **TCP 4415–4416** ve **UDP 4415** portlarını açın.

### İstemci — gelen port gerekmez

İstemci yalnızca giden bağlantılar kurar. İstemci tarafında herhangi bir güvenlik duvarı veya port yönlendirme kuralına gerek yoktur.

### Yerel ağ keşfi

mDNS otomatik keşfini kullanıyorsanız (kurulumda `Ctrl+F`), **5353 portunda UDP multicast**'e izin verin. Bu özellik `--no-discovery` ile devre dışı bırakılabilir.

### NAT geçişi (STUN / TURN)

EchoWarp, NAT arkasından bağlantı kurmak için STUN sunucularını kullanır. Her iki taraf da simetrik NAT arkasındaysa ve doğrudan bağlantı kurulamıyorsa, bir TURN aktarma sunucusu yapılandırılabilir (config dosyasında `turn_servers`). Hem STUN hem de TURN **giden** bağlantılardır ve herhangi bir gelen güvenlik duvarı kuralı gerektirmez.

## Lisans

[MIT](../../LICENSE) — üçüncü taraf lisansları için [THIRD_PARTY_NOTICES.md](../../THIRD_PARTY_NOTICES.md) dosyasına bakın.
