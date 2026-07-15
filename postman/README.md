# API Documentation (Postman + Swagger)

Proyek ini menyediakan dokumentasi API dalam dua format.

## 1. Swagger UI (Interaktif)

Jalankan server, lalu buka browser:

```
http://localhost:8080/swagger/index.html
```

- Tombol **Authorize** (🔒) di kanan atas → masukkan `Bearer <token>` (token didapat dari endpoint login).
- Tiap endpoint bisa di-try langsung dari UI (Try it out).
- Schema (request/response model) auto-generated dari struct Go + annotation.

Endpoint JSON mentah (untuk import ke tooling lain):

```
http://localhost:8080/swagger/doc.json
```

## 2. Postman Collection

Import dua file ini ke Postman:

| File | Tipe |
|------|------|
| `postman/ledger-backend-go.postman_collection.json` | Collection (semua endpoint) |
| `postman/ledger-backend-go.postman_environment.json` | Environment (`base_url`, dll.) |

### Cara Import

1. Buka Postman → **Import** → drag & drop kedua file.
2. Pilih environment **"Ledger Backend Go - Local"** di kanan atas.
3. Pastikan `JWT_SECRET` di `.env` server sudah ter-set.

### Alur Penggunaan (Recommended)

Jalankan request **berurutan** dalam folder:

1. **1. Auth** → Register budi & andi → Login budi & andi
   - Login otomatis menyimpan token ke environment (`budi_token`, `andi_token`).
   - Register otomatis menyimpan `user_id` ke environment.
2. **2. Wallet (Budi)** → Get Dashboard → TopUp (IDR) → Transfer (IDR) → History
3. **3. Wallet (Andi)** → Get Dashboard → Transfer balik ke budi
4. **4. Exchange** → Get Rate (USDT_IDR) → Swap IDR to USDT (mengonversi Rupiah ke USDT dengan fee 0.5%)
5. **5. Crypto** → Get/Create Deposit Address (mengenerate EVM address) → Withdraw Crypto
6. **6. Fiat Withdrawal** → Withdraw Fiat ke Rekening Bank (disbursement)
7. **7. Webhooks** → Simulasi callback pembayaran Midtrans Snap & callback payout Iris
8. **8. Health & Docs** → Ping, Swagger UI, System Health Check (`GET /health`)


### Dummy Data

| Field | Value |
|-------|-------|
| budi email | `budi@mail.com` |
| budi password | `secret123` |
| andi email | `andi@mail.com` |
| andi password | `secret123` |

### Auto Variable

Environment otomatis terisi:

| Variable | Sumber |
|----------|--------|
| `budi_token` | dari response Login budi (`data.token`) |
| `andi_token` | dari response Login andi (`data.token`) |
| `budi_user_id` | dari response Register budi (`data.user_id`) |
| `andi_user_id` | dari response Register andi (`data.user_id`) |

### Catatan

- Tiap kali server di-restart, DB di-truncate → register & login ulang dari awal.
- Kalau endpoint register gagal karena user sudah ada (409), tetap bisa langsung login (token lama invalidated setelah restart, jadi login lagi aman).
