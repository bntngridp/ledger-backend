package domain

import (
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// UserRepository defines the data access contract for the User aggregate.
type UserRepository interface {
	GetUserByEmail(email string) (*User, error)
	GetUserByID(id uuid.UUID) (*User, error)
	GetUserByGoogleID(googleID string) (*User, error)
	CheckEmailExists(email string) (bool, error)
	CheckUsernameExists(username string) (bool, error)
	CreateUserWithWallet(user *User, wallet *Wallet) error
	UpdateUser(user *User) error
}

// WalletRepository defines the data access contract for Wallet and WalletBalance aggregates.
type WalletRepository interface {
	GetWalletByUserID(userID uuid.UUID) (*Wallet, error)
	GetWalletBalance(walletID uuid.UUID, assetSymbol string) (*WalletBalance, error)
	GetBalancesByWalletID(walletID uuid.UUID) ([]WalletBalance, error)
	// GetOrCreateBalance fetches a balance row or initializes it at zero if it doesn't exist.
	GetOrCreateBalance(walletID uuid.UUID, assetSymbol string) (*WalletBalance, error)
}

// TransactionRepository defines the data access contract for financial operations.
// All methods that modify balances must be implemented with ACID database transactions
// and pessimistic locking (SELECT ... FOR UPDATE) to prevent race conditions.
type TransactionRepository interface {
	// Fiat / Transfer
	ExecuteTransferTx(senderWalletID, recipientWalletID uuid.UUID, amount decimal.Decimal, assetSymbol, notes string) error
	ExecuteTopUpTx(walletID uuid.UUID, amount decimal.Decimal, assetSymbol, notes string) (*Transaction, decimal.Decimal, error)

	// Midtrans / Webhook
	CreatePendingTopUpTx(walletID uuid.UUID, amount decimal.Decimal, assetSymbol, orderID, notes string) (*Transaction, error)
	SettleTopUpTx(transactionID, walletID uuid.UUID, amount decimal.Decimal) error
	GetTransactionByOrderID(orderID string) (*Transaction, error)
	UpdateTransactionStatus(txID uuid.UUID, status, notes string) error

	// History with Pagination
	GetTransactionsByWalletID(walletID uuid.UUID, page, perPage int, assetFilter, typeFilter string) ([]Transaction, int64, error)

	// Fiat Withdrawal (Midtrans Iris Disbursement)
	ExecuteWithdrawFiatTx(walletID uuid.UUID, amount, adminFee decimal.Decimal, assetSymbol, notes string) (*Transaction, error)

	// Crypto Deposit (credited by On-Chain Listener)
	CreditCryptoDeposit(walletID uuid.UUID, amount decimal.Decimal, assetSymbol, txHash, notes string) (*Transaction, error)

	// Crypto Withdrawal
	CreatePendingCryptoWithdrawTx(walletID uuid.UUID, amount decimal.Decimal, assetSymbol, toAddress, notes string) (*Transaction, error)
	UpdateCryptoWithdrawTx(txID uuid.UUID, txHash, status string) error
	RejectWithdrawCryptoTx(txID uuid.UUID, reason string) error

	// Swap
	ExecuteSwapTx(walletID uuid.UUID, fromAsset, toAsset string, fromAmount, toAmount, rateUsed, feeCharged decimal.Decimal) (*Transaction, error)

	// Reject Fiat Withdrawal (Refund)
	RejectWithdrawFiatTx(txID uuid.UUID, reason string) error
}

// CryptoAddressRepository defines the data access contract for on-chain deposit addresses.
type CryptoAddressRepository interface {
	// GetAddressByWalletID retrieves the deposit address for a given wallet, network, and asset.
	GetAddressByWalletID(walletID uuid.UUID, network, assetSymbol string) (*CryptoAddress, error)
	// GetAddressByValue looks up which user owns a given public address (used by the on-chain listener).
	GetAddressByValue(address string) (*CryptoAddress, error)
	// CreateAddress persists a new deposit address record with an encrypted private key.
	CreateAddress(cryptoAddr *CryptoAddress) error
	// GetAllAddresses returns all deposit addresses (used by on-chain listener to build a watch list).
	GetAllAddresses(network string) ([]CryptoAddress, error)
}
