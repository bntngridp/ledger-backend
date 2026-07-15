package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

type ErrorResponse struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

type SuccessResponse struct {
	Status  int         `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ============================================================
// Auth DTOs
// ============================================================

type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Token     string `json:"token"`
	ExpiresIn int    `json:"expires_in"`
}

// ============================================================
// Wallet / Dashboard DTOs
// ============================================================

type WalletBalanceDTO struct {
	AssetSymbol string          `json:"asset_symbol"`
	Balance     decimal.Decimal `json:"balance"`
}

type RegisterResponse struct {
	UserID   string             `json:"user_id"`
	Username string             `json:"username"`
	Email    string             `json:"email"`
	WalletID string             `json:"wallet_id"`
	Balances []WalletBalanceDTO `json:"balances"`
}

type DashboardResponse struct {
	WalletID          string             `json:"wallet_id"`
	Balances          []WalletBalanceDTO `json:"balances"`
	EstimatedTotalIDR decimal.Decimal    `json:"estimated_total_idr"`
}

// ============================================================
// Fiat / Transfer DTOs
// ============================================================

type TransferRequest struct {
	DestinationUserID string          `json:"destination_user_id" binding:"required"`
	AssetSymbol       string          `json:"asset_symbol" binding:"required"`
	Amount            decimal.Decimal `json:"amount" binding:"required,gt=0"`
	Notes             string          `json:"notes"`
}

type TopUpRequest struct {
	Amount decimal.Decimal `json:"amount" binding:"required,gt=0"`
	Notes  string          `json:"notes"`
}

type TopUpResponse struct {
	TransactionID string          `json:"transaction_id"`
	WalletID      string          `json:"wallet_id"`
	AssetSymbol   string          `json:"asset_symbol"`
	Amount        decimal.Decimal `json:"amount"`
	SnapToken     string          `json:"snap_token,omitempty"`
	RedirectURL   string          `json:"redirect_url,omitempty"`
}

type WithdrawFiatRequest struct {
	Amount        decimal.Decimal `json:"amount" binding:"required,gt=0"`
	BankCode      string          `json:"bank_code" binding:"required"`
	AccountNumber string          `json:"account_number" binding:"required"`
	AccountName   string          `json:"account_name" binding:"required"`
	Notes         string          `json:"notes"`
}

type WithdrawFiatResponse struct {
	TransactionID string          `json:"transaction_id"`
	Amount        decimal.Decimal `json:"amount"`
	AdminFee      decimal.Decimal `json:"admin_fee"`
	TotalDeducted decimal.Decimal `json:"total_deducted"`
	BankCode      string          `json:"bank_code"`
	AccountNumber string          `json:"account_number"`
	Status        string          `json:"status"`
}

// ============================================================
// Transaction History DTOs
// ============================================================

type PaginationMeta struct {
	Page       int   `json:"page"`
	PerPage    int   `json:"per_page"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

type TransactionHistoryItem struct {
	TransactionID       string           `json:"transaction_id"`
	SourceWalletID      *string          `json:"source_wallet_id"`
	DestinationWalletID *string          `json:"destination_wallet_id"`
	AssetSymbol         string           `json:"asset_symbol"`
	Amount              decimal.Decimal  `json:"amount"`
	Type                string           `json:"type"`
	Status              string           `json:"status"`
	TransactionNotes    string           `json:"transaction_notes"`
	TxHash              *string          `json:"tx_hash,omitempty"`
	MidtransOrderID     *string          `json:"midtrans_order_id,omitempty"`
	RateUsed            *decimal.Decimal `json:"rate_used,omitempty"`
	FeeCharged          *decimal.Decimal `json:"fee_charged,omitempty"`
	CreatedAt           time.Time        `json:"created_at"`
}

type TransactionHistoryResponse struct {
	Transactions []TransactionHistoryItem `json:"transactions"`
	Meta         PaginationMeta           `json:"meta"`
}

// ============================================================
// Crypto DTOs
// ============================================================

type GetDepositAddressRequest struct {
	Network     string `json:"network" binding:"required"`
	AssetSymbol string `json:"asset_symbol" binding:"required"`
}

type DepositAddressResponse struct {
	Address     string `json:"address"`
	Network     string `json:"network"`
	AssetSymbol string `json:"asset_symbol"`
}

type CryptoWithdrawRequest struct {
	AssetSymbol string          `json:"asset_symbol" binding:"required"`
	Network     string          `json:"network" binding:"required"`
	ToAddress   string          `json:"to_address" binding:"required"`
	Amount      decimal.Decimal `json:"amount" binding:"required,gt=0"`
	Notes       string          `json:"notes"`
}

type CryptoWithdrawResponse struct {
	TransactionID string          `json:"transaction_id"`
	TxHash        *string         `json:"tx_hash,omitempty"`
	AssetSymbol   string          `json:"asset_symbol"`
	Amount        decimal.Decimal `json:"amount"`
	ToAddress     string          `json:"to_address"`
	Status        string          `json:"status"`
}

// ============================================================
// Exchange DTOs
// ============================================================

type ExchangeRateResponse struct {
	Pair        string          `json:"pair"`
	Rate        decimal.Decimal `json:"rate"`
	LastUpdated time.Time       `json:"last_updated"`
}

type SwapRequest struct {
	FromAsset string          `json:"from_asset" binding:"required"`
	ToAsset   string          `json:"to_asset" binding:"required"`
	Amount    decimal.Decimal `json:"amount" binding:"required,gt=0"`
}

type SwapResponse struct {
	TransactionID string          `json:"transaction_id"`
	FromAsset     string          `json:"from_asset"`
	ToAsset       string          `json:"to_asset"`
	FromAmount    decimal.Decimal `json:"from_amount"`
	ToAmount      decimal.Decimal `json:"to_amount"`
	RateUsed      decimal.Decimal `json:"rate_used"`
	FeeCharged    decimal.Decimal `json:"fee_charged"`
}

// ============================================================
// Google OAuth DTOs
// ============================================================

type GoogleUserProfile struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
	Locale        string `json:"locale"`
}

// ============================================================
// Midtrans Iris Callback DTOs
// ============================================================

type IrisCallbackItem struct {
	PayoutID           int64   `json:"payout_id"`
	ReferenceNo        string  `json:"reference_no"`
	Amount             string  `json:"amount"`
	BeneficiaryName    string  `json:"beneficiary_name"`
	BeneficiaryAccount string  `json:"beneficiary_account"`
	BeneficiaryBank    string  `json:"beneficiary_bank"`
	Status             string  `json:"status"` // "completed" or "failed"
	CreatedAt          string  `json:"created_at"`
	UpdatedAt          string  `json:"updated_at"`
	ErrorMessage       *string `json:"error_message"`
}

