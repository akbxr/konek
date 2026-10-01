# Konek

Konek adalah daemon bot Telegram yang menghubungkan ponsel ke Herdr dan coding agent di komputer lokal (OMP, Claude Code, Codex, Pi). Bot ini tidak membutuhkan port forwarding, IP publik, atau konfigurasi VPN karena berkomunikasi keluar melalui HTTPS long polling.

## Cara kerja

- Komunikasi dua arah lewat Telegram API. Bot berjalan di komputer lokal dan menarik pesan dari server Telegram. Tidak ada port masuk yang dibuka di jaringan lokal.
- Terhubung langsung ke Herdr socket. Bot memantau siklus hidup agent (`idle`, `working`, `blocked`, `done`) dan mengirim perintah ke pane yang sesuai.
- Dukungan interaksi saat agent tertahan (`blocked`). Jika agent meminta izin menjalankan perintah shell atau menulis berkas, bot memunculkan tombol Approve, Reject, dan Stop di Telegram.
- Pembacaan transcript penuh. Untuk agent yang mencatat session (seperti OMP dan Pi), bot membaca jawaban lengkap dari file `.jsonl`, bukan memotong buffer terminal yang tergulung.
- Pembagian pesan panjang otomatis. Respons yang melebihi batas 4.096 karakter Telegram dipotong per paragraf dan dikirim berurutan.
- Eksekusi paralel. Beberapa agent dapat bekerja bersamaan. Setiap giliran kerja memiliki pesan pemantauan tersendiri dengan status terpisah.
- Pembatasan akses berbasis ID. Bot hanya merespons Telegram User ID yang terdaftar dalam konfigurasi whitelist. Pesan lain langsung ditolak.

## Kebutuhan sistem

- Go 1.22 atau lebih baru
- Herdr terpasang dan server berjalan (`herdr status`)
- Setidaknya satu coding agent (OMP, Claude Code, Codex, atau Pi)

## Instalasi dan setup

### 1. Dapatkan token bot dan user ID

1. Buka `@BotFather` di Telegram, kirim `/newbot`, dan simpan token bot yang diberikan.
2. Buka `@userinfobot` di Telegram untuk melihat ID numerik akun Anda (contoh: `123456789`).

### 2. Konfigurasi environment

Salin file contoh konfigurasi:

```bash
cp .env.example .env
```

Sesuaikan isi file `.env`:

```env
TELEGRAM_BOT_TOKEN="token_dari_botfather"
TELEGRAM_ALLOWED_USER_IDS="123456789"
HERDR_BIN_PATH="/Users/akbar/.local/bin/herdr"
DEFAULT_CWD="/Users/akbar/Code/projects"
```

Jika ada lebih dari satu user yang diizinkan, pisahkan ID dengan koma pada `TELEGRAM_ALLOWED_USER_IDS`.

### 3. Kompilasi dan jalankan

```bash
go build -o konek .
./konek
```

Ketik `/start` di Telegram untuk membuka menu navigasi.

## Perintah dan penggunaan

### Interaksi dengan agent

- Kirim pesan teks langsung. Pesan diteruskan sebagai instruksi ke agent yang sedang aktif.
- `@nama prompt`. Mengirim instruksi ke agent tertentu tanpa mengganti target utama. Nama dapat berupa label workspace, nama folder project, atau ID pane (contoh: `@keep-silent perbaiki validasi token` atau `@w5:p1 jalankan build`).
- Kirim foto atau screenshot. Bot mengunduh gambar ke penyimpanan lokal dan meneruskannya ke agent multimodal. Tulis instruksi pada kolom caption foto.
- `/jobs`. Menampilkan daftar pekerjaan paralel yang sedang aktif beserta durasi dan tombol pembatalan.
- `/broadcast <instruksi>`. Mengirim instruksi yang sama ke seluruh agent aktif secara serentak.
- `/agents`. Menampilkan daftar agent yang sedang berjalan untuk dipilih sebagai target aktif.
- `/status`. Menampilkan detail status agent yang sedang aktif (nama project, harness, status, direktori kerja).
- `/read [N]`. Tanpa angka, perintah ini mengambil jawaban lengkap terakhir dari session transcript. Dengan angka (misal `/read 50`), bot membaca N baris log terminal aktif.
- `/stop`. Mengirim sinyal Ctrl+C ke agent aktif untuk membatalkan proses yang sedang berjalan.
- `/keys <key>`. Mengirim tombol tertentu ke terminal agent (contoh: `/keys enter`, `/keys esc`, `/keys y`, `/keys n`).

### Pengelolaan workspace dan terminal

- `/workspaces`. Menampilkan daftar workspace Herdr sebagai tombol interaktif. Memilih workspace akan membuka daftar seluruh panel di dalamnya.
- `/newworkspace <nama> [folder]`. Membuat workspace baru di Herdr. Jika folder tidak ditulis, direktori otomatis dibuat di bawah `DEFAULT_CWD/<nama>`. Bot langsung menawarkan pilihan untuk menjalankan OMP, Claude Code, Codex, atau Pi di panel tersebut.
- `/split [right|down]`. Membagi panel aktif secara horizontal atau vertikal, lalu menampilkan menu untuk menjalankan agent baru.
- `/sh <perintah>`. Menjalankan perintah shell langsung pada direktori project aktif (contoh: `/sh git status`, `/sh git diff`).
- `/img <path>`. Mengirim file gambar dari komputer lokal ke chat Telegram (contoh: `/img screenshot.png` atau path absolut).
- `/menu`. Menampilkan ulang keyboard menu bawah jika ditutup.

## Menjalankan di background dengan macOS launchd

Agar bot berjalan otomatis di latar belakang saat komputer menyala:

```bash
# Salin konfigurasi plist
cp dev.konek.bot.plist ~/Library/LaunchAgents/

# Muat dan jalankan service
launchctl load ~/Library/LaunchAgents/dev.konek.bot.plist
```

Untuk memeriksa status proses:

```bash
launchctl list | grep dev.konek.bot
```

Untuk menghentikan service:

```bash
launchctl unload ~/Library/LaunchAgents/dev.konek.bot.plist
```

Berkas log disimpan pada direktori project:
- `konek.log` untuk keluaran standar (stdout)
- `konek.error.log` untuk keluaran galat (stderr)
