<p align="center">
  <img src="../../build/icons/icon_512.png" alt="EchoWarp Logo" width="128">
</p>

<h1 align="center">EchoWarp</h1>

<p align="center">
  Truyền phát âm thanh mạng theo thời gian thực giữa các máy chủ
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

Thu âm trên một máy, phát lại trên máy khác — theo thời gian thực qua mạng. EchoWarp sử dụng WebRTC để truyền tải và Opus để nén, cung cấp âm thanh độ trễ thấp với mã hóa đầu cuối.

## Tính năng

- **Truyền phát 1:1** — máy chủ thu âm, máy khách phát (hoặc ngược lại)
- **Chế độ song công** — âm thanh hai chiều, cả hai bên đều nghe thấy nhau
- **Phát sóng (1:N)** — một máy chủ truyền phát đến nhiều máy khách
- **Hội nghị (N:N)** — trộn âm thanh nhiều thành viên với bản trộn cá nhân (tổng trừ bản thân)
- **Micro ảo** — âm thanh nhận được xuất hiện dưới dạng đầu vào micro (Discord, Zoom, OBS)
- **Khám phá LAN** — tự động phát hiện máy chủ qua mDNS
- **Ghi âm** — ghi WAV: bản trộn, từng track, hoặc cả hai
- **Tăng tốc SIMD** — AVX/SSE (x86) và NEON (ARM) cho việc trộn âm thanh
- **Mã hóa đầu cuối** — AES + DTLS/SRTP
- **Đa nền tảng** — macOS, Linux, Windows

## Bắt đầu nhanh

1. Tải xuống file nhị phân dựng sẵn từ trang [Releases](https://github.com/lHumaNl/EchoWarp/releases) (không cần phụ thuộc — opus được liên kết tĩnh)
2. Chạy `EchoWarp` — menu tương tác cho phép chọn Server, Client, Chẩn đoán và hơn thế nữa. Hoặc vào thẳng chế độ với `EchoWarp server` / `EchoWarp client`
3. Cấu hình mọi thứ trong TUI tương tác và nhấn Enter để bắt đầu

> **Người dùng Windows:** Gói phát hành bao gồm `EchoWarp Server.bat` và `EchoWarp Client.bat` — nhấp đúp vào file bạn cần. Không cần mở cmd.exe thủ công.

## TUI Tương tác

TUI là giao diện chính của EchoWarp. Chỉ cần chạy `EchoWarp server` hoặc `EchoWarp client` — màn hình thiết lập tương tác sẽ tự động mở.

### Màn hình thiết lập

Màn hình thiết lập có bố cục hai cột:

**Cột trái — Thiết bị âm thanh.** Liệt kê tất cả các thiết bị đầu vào/đầu ra khả dụng cùng các thông số (kênh, tần số lấy mẫu, độ sâu bit). Trong chế độ song công hoặc hội nghị, bạn gán vai trò thu âm `[C]` và phát lại `[P]` riêng biệt. Tạo thiết bị ảo có thể thực hiện trực tiếp từ danh sách.

**Cột phải — Cài đặt.** Tất cả các thông số phiên có thể cấu hình tại đây với xác thực nội tuyến:

| Cài đặt | Mô tả |
|---------|-------|
| Địa chỉ máy chủ | IP/tên máy chủ (chỉ dành cho máy khách) |
| Cổng | Cổng TCP (mặc định: 4415) |
| Mật khẩu | Mật khẩu xác thực |
| Chế độ | Thông thường / Đảo ngược / Song công |
| Số client tối đa | Số kết nối tối đa (chỉ máy chủ) |
| Tần số lấy mẫu | 8000 / 16000 / 24000 / 48000 Hz |
| Kênh | Mono / Stereo |
| Bitrate Opus | 16–512 kbps |
| Khử tiếng vọng | AEC cho chế độ song công |
| TLS | Bật/tắt cùng đường dẫn cert và key |
| Mức log | debug / info / warn / error |

Điều hướng giữa các cột bằng `Tab`, di chuyển qua các trường bằng phím mũi tên, và nhấn `Enter` để xác nhận và bắt đầu truyền phát.

### Khám phá LAN

Trên màn hình thiết lập máy khách, nhấn `Ctrl+F` để mở lớp phủ khám phá. EchoWarp sử dụng mDNS để tự động tìm máy chủ trên mạng cục bộ — chọn một máy chủ từ danh sách và các trường địa chỉ/cổng sẽ được điền tự động.

### Hồ sơ thiết bị

Nhấn `Ctrl+O` để mở lớp phủ hồ sơ. Lưu cấu hình thiết bị hiện tại dưới dạng hồ sơ có tên, tải hồ sơ đã lưu trước đó, hoặc xóa các hồ sơ không còn cần thiết. Hồ sơ ghi nhớ các gán thiết bị và vai trò.

### Màn hình truyền phát

Sau khi kết nối, TUI hiển thị trạng thái truyền phát theo thời gian thực:

- **Thống kê kết nối** — RTT, jitter, mất gói với thanh ngưỡng cho thấy mức độ gần giới hạn chất lượng
- **Chất lượng kết nối** — chỉ số tổng thể (Xuất sắc / Tốt / Trung bình / Kém) tính toán từ RTT, jitter và mất gói
- **Bộ phân tích phổ** — trực quan hóa tần số FFT theo thời gian thực của tín hiệu âm thanh
- **Đồng hồ VU** — đồng hồ mức âm thanh theo từng kênh
- **Bitrate** — hiển thị bitrate tải lên/tải xuống trực tiếp
- **Điều khiển thiết bị** — điều chỉnh âm lượng, tắt tiếng, chuyển đổi thiết bị
- **Bảng log** — chế độ xem log có thể cuộn và bật/tắt

### Phím tắt TUI

| Phím | Hành động |
|------|-----------|
| `Tab` | Chuyển cột (thiết lập) |
| `Ctrl+F` | Khám phá LAN (thiết lập máy khách) |
| `Ctrl+O` | Hồ sơ thiết bị (thiết lập) |
| `Enter` | Xác nhận và bắt đầu |
| `Ctrl+Q` | Thoát |
| `Ctrl+P` | Tạm dừng/tiếp tục truyền phát |
| `Ctrl+L` | Bật/tắt bảng log |
| `Ctrl+M` | Tắt/bật tiếng |
| `+` / `-` | Tăng/giảm âm lượng |
| `Ctrl+Up/Down` | Cuộn log |
| `Ctrl+R` | Bắt đầu/dừng ghi âm |
| `Ctrl+D` | Đuổi client (máy chủ) |
| `Ctrl+B` | Cấm client (máy chủ) |

## Các Chế Độ Truyền Phát

EchoWarp có bốn chế độ truyền phát. Chọn chế độ phù hợp với kịch bản của bạn — chế độ được chọn trong màn hình thiết lập TUI (phía máy chủ) hoặc tự động phát hiện (phía máy khách).

### Normal — Một Chiều: Máy Chủ Đến Máy Khách

Máy chủ thu âm và gửi đến tất cả các máy khách đã kết nối. Máy khách chỉ nghe.

```
Server [captures audio] ───→ Client 1 [plays audio]
                         ├──→ Client 2 [plays audio]
                         └──→ Client N [plays audio]
```

**Khi nào sử dụng:** phát nhạc/podcast đến người nghe, phát âm thanh hệ thống (YouTube, Spotify) sang phòng khác, gửi nguồn loopback đến máy từ xa.

**Thiết bị cần thiết:**

| Phía | Thu âm (đầu vào) | Phát lại (đầu ra) |
|------|:-:|:-:|
| Máy chủ | bắt buộc | — |
| Máy khách | — | bắt buộc |

**Số tham gia:** 1 máy chủ + 1…N máy khách (đặt *Số client tối đa* trong TUI).

---

### Reverse — Một Chiều: Máy Khách Đến Máy Chủ

Ngược lại với Normal. Máy khách thu âm và gửi đến máy chủ. Máy chủ chỉ nghe.

```
Client 1 [captures audio] ───→ Server [plays audio]
Client 2 [captures audio] ──┘
```

**Khi nào sử dụng:** dùng microphone từ xa trên máy chủ, thu âm thanh từ vị trí từ xa, gửi mic của máy khách đến loa máy chủ.

**Thiết bị cần thiết:**

| Phía | Thu âm (đầu vào) | Phát lại (đầu ra) |
|------|:-:|:-:|
| Máy chủ | — | bắt buộc |
| Máy khách | bắt buộc | — |

**Số tham gia:** 1 máy chủ + 1…N máy khách.

---

### Duplex — Hai Chiều

Cả hai bên thu âm và phát đồng thời. Mọi người đều nghe thấy nhau.

```
Server [mic + speakers] ⟷ Client 1 [mic + speakers]
                        ⟷ Client 2 [mic + speakers]
```

**Khi nào sử dụng:** cuộc gọi thoại giữa hai hoặc nhiều máy, liên lạc nội bộ giữa các phòng, phiên âm thanh cộng tác.

**Thiết bị cần thiết:**

| Phía | Thu âm (đầu vào) | Phát lại (đầu ra) |
|------|:-:|:-:|
| Máy chủ | bắt buộc | bắt buộc |
| Máy khách | bắt buộc | bắt buộc |

**Số tham gia:** 1 máy chủ + 1…N máy khách. Hỗ trợ AEC (khử tiếng vọng) để ngăn vòng lặp phản hồi.

---

### Conference — Trộn Đa Người Dùng (N:N)

Máy chủ đóng vai trò trung tâm trộn. Mỗi thành viên (máy chủ + máy khách) gửi âm thanh và nhận bản trộn cá nhân của **tất cả người khác** (tổng trừ bản thân). Điều này ngăn việc nghe thấy giọng của chính mình.

```
Client 1 ──┐              ┌──→ Client 1 (hears Server + 2 + 3)
Client 2 ──┤  Server hub  ├──→ Client 2 (hears Server + 1 + 3)
Client 3 ──┘              └──→ Client 3 (hears Server + 1 + 2)
```

**Khi nào sử dụng:** cuộc gọi nhóm với 3+ máy, buổi tập từ xa, hội nghị nhiều thành viên.

**Thiết bị cần thiết:**

| Phía | Thu âm (đầu vào) | Phát lại (đầu ra) |
|------|:-:|:-:|
| Máy chủ (chỉ hub) | — | — |
| Máy chủ (thành viên) | bắt buộc | bắt buộc |
| Máy khách | bắt buộc | bắt buộc |

Máy chủ có thể hoạt động như **chỉ hub** (không có âm thanh cục bộ — chỉ trộn cho máy khách) hoặc như thành viên đầy đủ với mic và loa riêng.

**Số tham gia:** 1 máy chủ + 2…N máy khách. Hỗ trợ ghi âm (bản trộn, từng track, hoặc cả hai), đuổi/cấm, và điều chỉnh âm lượng từng thành viên.

---

### Tóm Tắt Chế Độ

| | Normal | Reverse | Duplex | Conference |
|---|---|---|---|---|
| **Hướng** | Máy chủ → Máy khách | Máy khách → Máy chủ | Hai chiều | Tất cả ⟷ Tất cả |
| **Thiết bị máy chủ** | Chỉ thu âm | Chỉ phát lại | Cả hai | Cả hai (hoặc không nếu hub) |
| **Thiết bị máy khách** | Chỉ phát lại | Chỉ thu âm | Cả hai | Cả hai |
| **Số tham gia tối đa** | 1 + N | 1 + N | 1 + N | 1 + N |
| **Hỗ trợ AEC** | — | — | có | có |
| **Bản trộn cá nhân** | — | — | — | có |
| **Ghi âm** | — | — | — | có |
| **Cờ CLI** | *(mặc định)* | `-r` | `-X` | `--conference` |

> **Mẹo:** Số lượng máy khách được kiểm soát bởi cài đặt *Số client tối đa* trong TUI (mặc định: 1). Tăng lên nếu bạn cần kịch bản phát sóng hoặc đa người dùng.

## Hướng Dẫn Định Tuyến Âm Thanh

### Các Trường Hợp Sử Dụng

EchoWarp hỗ trợ một số kịch bản định tuyến âm thanh. Bảng dưới đây cho thấy chế độ và cấu hình thiết bị nào cần sử dụng cho từng trường hợp:

| Kịch bản | Chế độ | Thiết bị máy chủ | Thiết bị máy khách |
|----------|--------|------------------|---------------------|
| Phát mic đến loa từ xa | Normal | Mic `[Input]` | Loa `[Output]` |
| Phát âm thanh hệ thống (YouTube, Spotify) | Normal | 🔄 Loopback `[Input]` | Loa `[Output]` |
| Dùng mic từ xa trong Discord/Zoom | Reverse | Loa `[Output]` | Mic `[Input]` |
| Trò chuyện thoại hai chiều | Duplex | Mic + Loa | Mic + Loa |
| Cuộc gọi nhóm (3+ máy) | Conference | — (hub) | Mic + Loa |
| Âm thanh từ xa làm mic ảo trong OBS | Normal | 🔄 Loopback `[Input]` | ⟡ Virtual mic `[Output]` |
| Phát + mic cục bộ thành một mic ảo | Normal | Mic `[Input]` | ⟡ Virtual `[Output]` + 🎤 Mix mic |

### Loopback — Ghi Lại Âm Thanh Hệ Thống

Thiết bị loopback ghi lại toàn bộ âm thanh đang phát trên máy (nhạc, cuộc gọi video, âm thanh game) và cung cấp dưới dạng nguồn đầu vào.

| Nền tảng | Cách hoạt động | Thiết lập |
|----------|----------------|-----------|
| **macOS** | Aggregate Device (loa + BlackHole) | Cài [BlackHole](https://github.com/ExistentialAudio/BlackHole): `brew install blackhole-2ch`. Thiết bị loopback tự động xuất hiện trong TUI |
| **Windows** | WASAPI native loopback | Tích hợp sẵn, không cần phần mềm bổ sung |
| **Linux** | PulseAudio monitor sources | Tích hợp sẵn với PulseAudio/PipeWire |

Trong TUI, thiết bị loopback được đánh dấu bằng 🔄 và xuất hiện trong phần Input.

### Microphone Ảo — Định Tuyến Âm Thanh Đến Các Ứng Dụng Khác

Microphone ảo làm cho âm thanh nhận được xuất hiện dưới dạng đầu vào mic mà Discord, Zoom, OBS và các ứng dụng khác có thể sử dụng.

| Nền tảng | Driver | Cài đặt |
|----------|--------|---------|
| **macOS** | [BlackHole](https://github.com/ExistentialAudio/BlackHole) | `brew install blackhole-2ch` |
| **Windows** | [VB-Audio Virtual Cable](https://vb-audio.com/Cable/) | Tải xuống từ vb-audio.com |
| **Linux** | PulseAudio null-sink | Tạo từ TUI: Settings → Virtual mic → Create |

Trên macOS và Windows, cài driver và chọn nó trong phần Output của TUI. Trên Linux, TUI tự động tạo virtual sink — trong các ứng dụng khác, chọn **"Monitor of EchoWarp"** làm microphone.

Thiết bị ảo được đánh dấu bằng ⟡ trong TUI và hiển thị **adaptive** thay vì tần số lấy mẫu cố định.

### Trộn Microphone Cục Bộ Vào Đầu Ra Ảo

Khi bạn định tuyến luồng từ xa đến thiết bị đầu ra ảo (BlackHole, VB-Cable), các ứng dụng bên thứ ba như Discord hoặc Zoom chỉ nghe thấy luồng đó — chúng không nghe thấy microphone cục bộ của bạn. Nếu bạn cần cả giọng nói của mình **và** luồng từ xa xuất hiện dưới dạng một đầu vào mic duy nhất, EchoWarp có thể trộn chúng lại với nhau.

**Cách sử dụng:** Trong TUI, chọn thiết bị ảo trong phần Output. Các thiết bị đầu vào sẽ xuất hiện bên dưới dưới dạng mục con — đánh dấu microphone bạn muốn trộn vào:

```
🔊 Output
  ⟡ BlackHole 2ch                           stereo 32-bit adaptive   [✓]
      └─ 🎤 Built-in Microphone                                      [✓]
```

Âm thanh đã trộn (luồng + mic) được ghi vào đầu ra ảo. Trong Discord/Zoom/OBS, chọn thiết bị ảo đó làm microphone — cả luồng từ xa và giọng nói của bạn đều sẽ được nghe thấy.

**Các phương án thay thế ở cấp hệ điều hành** (không sử dụng bộ trộn tích hợp):

| Nền tảng | Cách thực hiện | Chi tiết |
|----------|----------------|----------|
| **macOS** | Aggregate Device | Mở Audio MIDI Setup → Tạo Aggregate Device kết hợp mic + BlackHole. Chọn aggregate đó làm đầu vào trong ứng dụng |
| **Windows** | [VoiceMeeter](https://vb-audio.com/Voicemeeter/) | Bộ trộn ảo miễn phí — định tuyến cả VB-Cable và mic vào VoiceMeeter, dùng đầu ra của nó làm mic trong ứng dụng |
| **Linux** | PulseAudio combine-source | `pactl load-module module-combine-source source_name=combined slaves=<mic>,<virtual>.monitor` |

Bộ trộn tích hợp hoạt động trên tất cả các nền tảng và không yêu cầu cấu hình bổ sung.

### Các Phần Thiết Bị

TUI chỉ hiển thị các phần liên quan đến chế độ hiện tại:

| Chế độ + Vai trò | Các phần hiển thị |
|------------------|------------------|
| Normal server / Reverse client | 🎤 Chỉ Input |
| Normal client / Reverse server | 🔊 Chỉ Output |
| Duplex / Conference | 🎤 Input + 🔊 Output |

### Chẩn Đoán

Chạy `EchoWarp doctor` để kiểm tra cấu hình âm thanh của bạn:

```
Audio
[OK] Audio system: 2 input device(s), 2 output device(s)
[OK] Virtual audio: BlackHole 2ch detected
```

Nếu không tìm thấy driver âm thanh ảo, doctor sẽ hiển thị hướng dẫn cài đặt cho nền tảng của bạn.

### Câu Hỏi Thường Gặp

**Tôi muốn nghe nhạc từ máy tính của mình trên loa ở phòng khác.**
→ Chạy `EchoWarp server` trên máy phát nhạc, chọn thiết bị 🔄 loopback (loa của bạn) trong Input. Chạy `EchoWarp client` trên máy từ xa, chọn loa trong Output.

**Tôi muốn sử dụng microphone từ xa cho Zoom/Discord.**
→ Chạy `EchoWarp server` ở chế độ Reverse trên máy từ xa (máy có mic). Chạy `EchoWarp client` trên máy của bạn, chọn thiết bị âm thanh ảo (BlackHole/VB-Cable) trong Output. Trong Zoom/Discord, chọn thiết bị ảo đó làm microphone.

**Tôi muốn phát âm thanh hệ thống (YouTube, Spotify) sang máy khác.**
→ Giống như "nhạc ở phòng khác" — dùng 🔄 loopback ở phía máy chủ. Thiết bị loopback ghi lại tất cả âm thanh đang phát qua loa đã chọn.

**Tôi muốn âm thanh từ xa xuất hiện dưới dạng đầu vào mic trong OBS.**
→ Chạy máy chủ với nguồn âm thanh. Trên máy của bạn (client), chọn thiết bị âm thanh ảo trong Output. Trong OBS, thêm nguồn "Audio Input Capture" và chọn thiết bị ảo.

**Tôi muốn trò chuyện thoại hai chiều giữa hai máy tính.**
→ Dùng chế độ Duplex. Trên cả hai máy, chọn microphone trong Input và loa trong Output.

**Tôi muốn cuộc gọi nhóm với 3+ máy.**
→ Dùng chế độ Conference. Máy chủ đóng vai trò hub (không cần thiết bị). Mỗi client chọn mic và loa. Mọi người đều nghe được nhau, trừ giọng của chính mình.

**Tôi muốn Discord nghe cả luồng từ xa VÀ giọng nói của tôi qua một mic ảo.**
→ Trên máy khách, chọn thiết bị ảo (BlackHole/VB-Cable) trong Output. Danh sách thiết bị đầu vào sẽ xuất hiện bên dưới — đánh dấu microphone của bạn. EchoWarp sẽ trộn luồng từ xa và mic của bạn vào đầu ra ảo. Trong Discord, chọn thiết bị ảo đó làm microphone.

**Tôi chưa cài BlackHole / VB-Cable.**
→ Chạy `EchoWarp doctor` — nó sẽ cho bạn biết chính xác cần cài gì và cài như thế nào. Trên Linux, không cần phần mềm bổ sung.

**Thiết bị loopback không xuất hiện trong TUI.**
→ macOS: Cài BlackHole trước (`brew install blackhole-2ch`). Windows/Linux: thiết bị loopback được tích hợp sẵn và sẽ tự động xuất hiện.

**Thiết bị ảo hiển thị "adaptive" thay vì tần số lấy mẫu.**
→ Đây là bình thường. Driver âm thanh ảo (BlackHole, VB-Cable) tự thích nghi với tần số lấy mẫu mà ứng dụng sử dụng — tốc độ hiển thị không có ý nghĩa thực tế.

## Chế độ CLI

Tất cả các cài đặt có trong TUI cũng có thể được truyền dưới dạng cờ CLI để viết script và tự động hóa:

```bash
# Máy chủ
EchoWarp server -d 1 -P mypassword

# Máy khách (tự động khám phá máy chủ trên LAN nếu -a bị bỏ qua)
EchoWarp client -a 192.168.1.10 -d 2 -P mypassword

# Liệt kê các thiết bị âm thanh
EchoWarp devices
```

### Các chế độ

Xem [Các Chế Độ Truyền Phát](#các-chế-độ-truyền-phát) để biết mô tả chi tiết từng chế độ.

| Chế độ | Cờ |
|--------|-----|
| Thông thường | *(mặc định)* |
| Đảo ngược | `-r` |
| Song công | `-X` |
| Hội nghị | `--conference` |

### Các cờ thông dụng

| Cờ | Viết tắt | Mô tả |
|----|----------|-------|
| `--device` | `-d` | ID thiết bị âm thanh |
| `--device-name` | `-D` | Chọn thiết bị theo tên (khớp chuỗi con) |
| `--password` | `-P` | Mật khẩu xác thực |
| `--port` | `-p` | Cổng TCP (mặc định: 4415) |
| `--sample-rate` | | Tần số lấy mẫu (mặc định: 48000) |
| `--channels` | | 1=mono, 2=stereo (mặc định: 1) |
| `--max-clients` | | Số client kết nối tối đa (mặc định: 1) |
| `--virtual-mic` | | Tạo thiết bị micro ảo |
| `--config` | `-c` | Tải file cấu hình YAML |
| `--save-config` | `-s` | Lưu cài đặt hiện tại vào YAML |
| `--profile` | | Tải hồ sơ thiết bị đã lưu |
| `--stun-server` | | Máy chủ STUN tùy chỉnh để duyệt NAT |
| `--tls-cert` / `--tls-key` | | Chứng chỉ và khóa TLS |
| `--log-level` | | debug, info, warn, error |
| `--aec` | | Khử tiếng vọng âm học (song công) |
| `--loopback` | | Thu âm hệ thống (macOS, yêu cầu BlackHole) |
| `--dry-run` | | Xác thực cấu hình và thoát |

### File cấu hình

Ưu tiên cài đặt: Cờ CLI > biến môi trường (`ECHOWARP_*`) > file cấu hình > mặc định.

Màn hình cài đặt TUI cho phép bạn lưu và tải file cấu hình một cách tương tác. Bạn cũng có thể sử dụng cờ CLI:

```bash
# Lưu cài đặt hiện tại vào file
EchoWarp server -d 1 -P pass --channels 2 -s myconfig.yml

# Tải cài đặt từ file
EchoWarp server -c myconfig.yml
```

## Cài đặt

### File nhị phân dựng sẵn

Tải xuống từ trang [Releases](https://github.com/lHumaNl/EchoWarp/releases). Có sẵn cho:
- **macOS** — arm64 (Apple Silicon), amd64 (Intel), bao gồm gói `.app`
- **Linux** — amd64, arm64, armv7, armv6
- **Windows** — amd64, arm64

### Xây dựng từ mã nguồn

Yêu cầu Go 1.22+ và header phát triển libopus.

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

File nhị phân kết quả liên kết tĩnh opus — không cần phụ thuộc runtime.

## Yêu cầu hệ thống

- **macOS** 11.0+ (arm64, amd64)
- **Linux** (amd64, arm64, armv7, armv6)
- **Windows** 10+ (amd64, arm64)

Đối với micro ảo:
- **macOS**: Driver âm thanh [BlackHole](https://github.com/ExistentialAudio/BlackHole)
- **Windows**: [VB-Audio Virtual Cable](https://vb-audio.com/Cable/)
- **Linux**: PulseAudio (`pactl`)

## Mạng & Tường lửa

### Máy chủ — các cổng cần mở

| Cổng | Giao thức | Mục đích |
|------|-----------|----------|
| `4415` | **TCP** | Tín hiệu (xác thực + bắt tay WebRTC) |
| `4415` | **UDP** | Phương tiện âm thanh (WebRTC). Ghép kênh — một cổng phục vụ tất cả máy khách |
| `4416` | **TCP** | Thăm dò thông tin phiên (tự động cấu hình máy khách) |

> Cổng `4415` là mặc định và có thể thay đổi bằng `--port`. Cổng thông tin phiên luôn là `port + 1`.

**Tóm lại:** mở **TCP 4415–4416** và **UDP 4415** chiều đến trên máy chủ.

### Máy khách — không cần mở cổng đến

Máy khách chỉ thực hiện kết nối đi. Không cần quy tắc tường lửa hay chuyển tiếp cổng ở phía máy khách.

### Khám phá mạng LAN

Nếu bạn sử dụng tự động khám phá qua mDNS (`Ctrl+F` trong cài đặt), hãy cho phép **multicast UDP trên cổng 5353**. Tính năng này có thể tắt bằng `--no-discovery`.

### Xuyên NAT (STUN / TURN)

EchoWarp sử dụng máy chủ STUN để thiết lập kết nối qua NAT. Nếu cả hai bên đều nằm sau NAT đối xứng và không thể thiết lập kết nối trực tiếp, có thể cấu hình máy chủ chuyển tiếp TURN (`turn_servers` trong config). Cả STUN và TURN đều là kết nối **đi** và không yêu cầu bất kỳ quy tắc tường lửa chiều đến nào.

## Giấy phép

[MIT](../../LICENSE) — xem [THIRD_PARTY_NOTICES.md](../../THIRD_PARTY_NOTICES.md) cho giấy phép của bên thứ ba.
