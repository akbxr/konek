# Konek ⚡

Bot Telegram super ringan (~8 MB binary, ~10 MB RAM) berbasis **Go** untuk mengendalikan **Herdr** dan coding agent **OMP** langsung dari Telegram tanpa perlu port forwarding atau ekspos SSH publik.

---

## 🌟 Kenapa Menggunakan Arsitektur Ini?

1. **Zero Open Ports / Zero NAT Traversal**: Bot berjalan di perangkat lokal dan berkomunikasi dengan Telegram via **Long Polling** (outbound HTTPS). Anda tidak perlu membuka port SSH di router rumah/kantor.
2. **Native Herdr + OMP Integration**: Berinteraksi langsung dengan Herdr API Socket/CLI untuk mendeteksi lifecycle OMP (`idle`, `working`, `blocked`, `done`).
3. **Interactive & Approval Ready**: Jika OMP meminta konfirmasi/approval (status `blocked`), bot otomatis memunculkan tombol Telegram:
   - `[ ✅ Approve (Enter) ]`
   - `[ ❌ Reject (n) ]`
   - `[ 🛑 Stop (Ctrl+C) ]`
4. **Live Progress & Full Markdown Transcript**: Status pekerjaan OMP dipantau real-time, dan hasil akhir dikirimkan lengkap sesuai format Markdown asli OMP (bukan potongan terminal yang ter-scroll).
5. **Human-Friendly Display Names**: Menampilkan nama project dan judul task yang bersih (contoh: `keep-silent: Roblox Asset Privacy and Spawning`), bukan kode teknis pane seperti `w5:p1`.
6. **Strict Security Whitelist**: Hanya Telegram User ID yang terdaftar yang dapat memberikan instruksi ke komputer Anda.

---

## 🚀 Persiapan & Menjalankan

### 1. Dapatkan Bot Token & User ID Telegram
1. Buka Telegram dan cari **`@BotFather`**, kirim `/newbot`, lalu ikuti langkahnya untuk mendapatkan `TELEGRAM_BOT_TOKEN`.
2. Cari bot **`@userinfobot`** di Telegram untuk melihat numeric ID akun Telegram Anda (misal: `123456789`).

### 2. Salin dan Konfigurasi `.env`
```bash
cp .env.example .env
```
Edit file `.env`:
```env
TELEGRAM_BOT_TOKEN="token_dari_botfather_disini"
TELEGRAM_ALLOWED_USER_IDS="id_angka_anda_disini"
HERDR_BIN_PATH="/Users/akbar/.local/bin/herdr"
DEFAULT_CWD="/Users/akbar/Code/projects"
```

### 3. Build & Jalankan
```bash
# Build binary
go build -o konek .

# Jalankan secara interaktif
./konek
```

---

## 📱 Cara Penggunaan di Telegram

- **Kirim Pesan Biasa**:
  Ketik langsung apa yang ingin Anda kerjakan, misal:
  > *"tolong perbaiki bug di auth.ts dan tambahkan unit test"*
  Bot akan langsung meneruskannya ke agent OMP yang sedang aktif.

- **/agents**:
  Menampilkan daftar pane/agent OMP yang aktif di Herdr dalam bentuk tombol inline. Klik untuk berpindah agent yang ingin dikontrol.

- **/status**:
  Melihat status detail agent yang sedang aktif (workspace, CWD, judul terminal, dan state: `idle`/`working`/`blocked`).

- **/read [N]**:
  - `/read` (tanpa argumen): Menampilkan jawaban lengkap terakhir dari agent.
  - `/read [N]`: Membaca *N* baris log mentah output terminal agent (misal: `/read 50`).
- **/workspaces**:
  Melihat daftar workspace di Herdr beserta jumlah tab dan pane.

- **/sh `<command>`**:
  Menjalankan perintah shell langsung di host pada direktori project aktif (misal: `/sh git status`, `/sh git diff`, `/sh npm test`).

- **/keys `<key>`**:
  Mengirim tombol kontrol ke terminal agent (misal: `/keys enter`, `/keys esc`, `/keys ctrl+c`, `/keys y`).

- **/stop**:
  Shortcut mengirim sinyal `Ctrl+C` ke agent aktif untuk membatalkan proses yang sedang berjalan.

---

## 🔄 Menjalankan sebagai Background Service (macOS LaunchAgent)

Agar bot otomatis berjalan di background dan otomatis aktif saat komputer dinyalakan:

```bash
# 1. Salin plist ke folder LaunchAgents
cp dev.konek.bot.plist ~/Library/LaunchAgents/

# 2. Muat dan jalankan service
launchctl load ~/Library/LaunchAgents/dev.konek.bot.plist

# Untuk cek status:
launchctl list | grep dev.konek.bot

# Untuk menghentikan service:
launchctl unload ~/Library/LaunchAgents/dev.konek.bot.plist
```

Log aplikasi tersimpan di:
- `konek.log` (stdout)
- `konek.error.log` (stderr)
