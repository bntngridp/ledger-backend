# Ledger Backend — Hybrid Fintech & Crypto Wallet API

[![Go CI](https://github.com/bntngridp/ledger-backend/actions/workflows/go-ci.yml/badge.svg)](https://github.com/bntngridp/ledger-backend/actions/workflows/go-ci.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/bntngridp/ledger-backend)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Ledger Backend is a high-performance, transaction-safe **Hybrid Fintech & Crypto Wallet REST API** built in Go. It seamlessly integrates traditional fiat banking (Indonesian Rupiah / IDR) with EVM-compatible blockchain stablecoins (`USDT`, `USDC`), offering instant asset swapping, peer-to-peer transfers, real-time Web3 deposit listeners, and automated payment gateway processing.

Engineered with **Clean Architecture** principles, strict concurrency safeguards (`SELECT ... FOR UPDATE` pessimistic locking), and **ACID database transactions**, Ledger guarantees complete financial data integrity and zero race conditions under heavy load.

---

## 🌟 Key Features

### 1. 🔐 Authentication & Security Management
- **JWT Authentication**: Secure JSON Web Token auth flow with configurable expiry and token validation middleware.
- **Google OAuth 2.0 Integration**: One-click social sign-in using Google accounts.
- **Two-Factor Authentication (2FA / TOTP)**: Two-step verification powered by Time-Based One-Time Passwords (TOTP). Secret keys are encrypted at rest using **AES-256-GCM** and enforced on high-risk operations (transfers & withdrawals).
- **Email OTP & Password Security**: One-time email password reset and change password workflows.

### 2. 💵 Fiat Currency Engine (IDR)
- **Automated Top-Up (Midtrans Snap)**: Seamless IDR top-up invoice generation supporting Virtual Accounts (BCA, Mandiri, BNI, BRI, Permata) and QRIS e-wallets.
- **Webhook Payment Verification**: Real-time payment settlement processing secured by **SHA-512 Signature Key** verification (`SHA512(order_id + status_code + gross_amount + server_key)`).
- **P2P Fiat Transfer**: Instant peer-to-peer IDR transfers protected against double-spending and race conditions.
- **Fiat Disbursement (Midtrans Iris)**: Automated external bank payouts with automatic balance refund handlers upon payout failures.

### 3. 🪙 Crypto Asset Module (USDT & USDC)
- **Dynamic EVM Deposit Address Generation**: Generates unique deposit addresses on Polygon Amoy / Ethereum Sepolia testnets. Private keys are encrypted using **AES-256-GCM** and safely zeroed out from memory after usage.
- **On-Chain ERC-20 Listener**: Background Goroutine monitoring ERC-20 Transfer events via **Alchemy WebSocket RPC**, featuring automatic reconnection, minimum 3-block confirmations, and strict `tx_hash` idempotency.
- **On-Chain Crypto Withdrawal**: Autonomous EVM transaction building, signing, and broadcasting to external crypto wallets.

### 4. 💱 Instant Asset Swap Engine
- **Binance Spot Rate Feed**: Fetches live exchange rates with an in-memory TTL cache to eliminate external API rate limiting.
- **Instant Swapping**: Zero-slippage conversions between IDR $\leftrightarrow$ Crypto or Crypto $\leftrightarrow$ Crypto with a configurable flat platform fee (default: 0.5%).

### 5. 🔔 Notifications & System Audit
- **In-App Notification Dispatcher**: System-triggered alerts for transfers, deposits, withdrawals, and security updates.
- **Read & Unread State Management**: Endpoints for unread badge counters, single mark-as-read, and batch mark-all-read.

---

## 🛠️ Architecture & Tech Stack

```text
Delivery (HTTP / REST Handlers)
      ↓
Use Case (Business Logic & Validation)
      ↓
Repository (Database I/O & External APIs)
      ↓
Domain (Entities & Interfaces)
```

- **Language**: Go 1.25+
- **HTTP Framework**: Gin v1.12
- **Database ORM**: GORM v1.31
- **Database Engine**: PostgreSQL 16
- **Blockchain Library**: `go-ethereum` (geth) v1.13
- **Payment Gateway**: Midtrans Snap & Iris Disbursement SDK
- **Market Data Feed**: Binance Spot Public API
- **API Documentation**: Swagger / OpenAPI 2.0 & Postman Collection

---

## 📋 Prerequisites

Before running the application, ensure you have the following installed:
- [Go 1.25+](https://go.dev/dl/)
- [Docker & Docker Compose](https://docs.docker.com/)
- An active [Alchemy API Account](https://www.alchemy.com/) (for Polygon Amoy testnet RPC)
- An active [Midtrans Sandbox Account](https://midtrans.com/) (Server Key & Client Key)

---

## 🚀 Quick Start Guide

### 1. Clone & Setup Environment
```bash
git clone https://github.com/bntngridp/ledger-backend.git
cd ledger-backend
cp .env.example .env
```

Edit `.env` and fill in your environment variables:
```env
PORT=8080
JWT_SECRET=your_jwt_secret_key_here
CRYPTO_ENCRYPTION_KEY=your_base64_32byte_aes_key
MIDTRANS_SERVER_KEY=SB-Mid-server-xxx
ALCHEMY_HTTP_URL=https://polygon-amoy.g.alchemy.com/v2/your_key
ALCHEMY_WS_URL=wss://polygon-amoy.g.alchemy.com/v2/your_key
```

### 2. Start PostgreSQL Database
```bash
docker compose up -d
```

### 3. Run the Backend API Server
```bash
go run ./cmd/api
```
The API server will start at `http://localhost:8080`.

---

## 📚 API Documentation

### 1. Interactive Swagger UI
Access the live interactive Swagger documentation by navigating to:
```text
http://localhost:8080/swagger/index.html
```

### 2. Postman Collection & Environment
Import the pre-configured Postman files located in the `/postman` directory:
- [`postman/ledger-backend-go.postman_collection.json`](file:///Users/bintang/Documents/Github/Ledger/ledger-backend/postman/ledger-backend-go.postman_collection.json)
- [`postman/ledger-backend-go.postman_environment.json`](file:///Users/bintang/Documents/Github/Ledger/ledger-backend/postman/ledger-backend-go.postman_environment.json)

> **Note**: All Postman requests include complete, pre-filled dummy data and automatically save JWT authentication tokens (`budi_token` & `andi_token`) for seamless out-of-the-box execution!

---

## 🧪 Testing

Run the test suite with race detector enabled:
```bash
go test -v -race ./...
```

To regenerate Swagger documentation:
```bash
swag init -g cmd/api/main.go
```

---

## 📄 License

This project is licensed under the [MIT License](LICENSE).
