# Dokumentasi API (Postman Collection & Swagger UI)

Proyek **Ledger Backend Go** menyediakan dua jenis dokumentasi API interaktif yang sudah dilengkapi dengan **Data Dummy Siap-Pakai (Ready to Run)**.

---

## 1. 🌐 Swagger UI (Dokumentasi Interaktif Web)

Akses Swagger UI melalui browser saat server backend aktif (`http://localhost:8080`):

```text
http://localhost:8080/swagger/index.html
```

### Fitur Swagger UI:
- **Autentikasi Bearer JWT**: Klik tombol **Authorize (🔒)** di sudut kanan atas $\rightarrow$ masukkan `Bearer <token>` (token didapat dari endpoint Login Budi/Andi).
- **Try It Out**: Semua endpoint mendukung eksekusi langsung dari browser dengan contoh schema request/response yang lengkap.
- **OpenAPI Schema (Raw JSON)**: Dapat diakses di `http://localhost:8080/swagger/doc.json`.

---

## 2. 🚀 Postman Collection (Siap-Pakai dengan Dummy Data)

Collection Postman telah dirancang sedemikian rupa sehingga **semua request sudah terisi data dummy secara otomatis**, dan token JWT (`budi_token` & `andi_token`) serta User ID akan tersimpan otomatis ke variabel environment tanpa perlu input manual.

### File Postman yang Disediakan:

| File | Fungsi |
| :--- | :--- |
| [`postman/ledger-backend-go.postman_collection.json`](file:///Users/bintang/Documents/Github/Ledger/ledger-backend/postman/ledger-backend-go.postman_collection.json) | Koleksi endpoint API lengkap (Auth, Wallet, Transfer, Swap, Crypto, Fiat Withdraw, Notifikasi, Webhooks) |
| [`postman/ledger-backend-go.postman_environment.json`](file:///Users/bintang/Documents/Github/Ledger/ledger-backend/postman/ledger-backend-go.postman_environment.json) | Variabel Environment (`base_url`, `budi_token`, `andi_user_id`, dll.) |

---

### 📥 Cara Import & Menjalankan di Postman

1. Buka aplikasi Postman.
2. Klik tombol **Import** $\rightarrow$ drag & drop file `ledger-backend-go.postman_collection.json` dan `ledger-backend-go.postman_environment.json`.
3. Pilih environment **"Ledger Backend Go - Local"** di dropdown kanan atas Postman.
4. **Tinggal Klik "Send"!** Seluruh request sudah dilengkapi contoh data dummy valid:

### 📋 Data Dummy Default yang Digunakan:

| Akun / Variabel | Email / Parameter Dummy | Password / Value |
| :--- | :--- | :--- |
| **User Budi** | `budi@mail.com` | `password123` |
| **User Andi** | `andi@mail.com` | `password123` |
| **Nominal Top-Up** | `250000` (IDR) | Midtrans Snap Integration |
| **Nominal Transfer P2P** | `50000` (IDR) | Budi $\rightarrow$ Andi |
| **Instant Swap** | `50000` IDR $\rightarrow$ USDT | Rates recalculation |
| **Withdrawal Fiat** | Bank BCA (`1234567890`) / DANA (`081234567890`) | Nominal: Rp 50.000 |
| **Withdrawal Crypto** | Polygon Amoy (`0x70997970C51812dc3A010C7d01b50e0d17dc79C8`) | Nominal: 1.5 USDT |

---

### 🔄 Alur Eksekusi Koleksi yang Direkomendasikan:

1. **`1. Auth & Security`**:
   - `Register Budi` $\rightarrow$ `Register Andi` $\rightarrow$ `Login Budi` $\rightarrow$ `Login Andi`
   *(Token JWT `budi_token` & `andi_token` otomatis tersimpan ke environment).*
2. **`2. Wallet & Dashboard (Budi)`**:
   - `Get Dashboard Summary` $\rightarrow$ `TopUp 250000 IDR (Midtrans Snap)` $\rightarrow$ `Get Transaction History`.
3. **`3. Transfer P2P`**:
   - `Transfer 50000 IDR (Budi to Andi)` $\rightarrow$ `Transfer 10000 IDR (Andi back to Budi)`.
4. **`4. Instant Swap / Exchange`**:
   - `Get Rate (USDT_IDR)` $\rightarrow$ `Swap 50000 IDR to USDT`.
5. **`5. Crypto Wallet`**:
   - `Get / Create Deposit Address (USDT)` $\rightarrow$ `Withdraw Crypto (USDT)`.
6. **`6. Fiat Withdrawal (Bank & E-Wallet)`**:
   - `Withdraw Fiat to Bank Account (BCA)` $\rightarrow$ `Withdraw Fiat to E-Wallet (DANA)`.
7. **`7. Notifications Center`**:
   - `Get All Notifications` $\rightarrow$ `Get Unread Count` $\rightarrow$ `Mark All Notifications as Read`.
8. **`8. Webhooks (Midtrans & Iris)`**:
   - Simulasi callback notifikasi pembayaran Midtrans Snap & Iris Payout.
