package repository

import (
	"errors"
	"fmt"
	"time"

	"github.com/bntngridp/ledger-backend/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type walletRepository struct {
	db *gorm.DB
}

func NewWalletRepository(db *gorm.DB) domain.WalletRepository {
	return &walletRepository{db: db}
}

func (r *walletRepository) GetWalletByUserID(userID uuid.UUID) (*domain.Wallet, error) {
	var wallet domain.Wallet
	if err := r.db.Preload("User").Preload("Balances").Preload("CryptoAddresses").Where("user_id = ?", userID).First(&wallet).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &wallet, nil
}

func (r *walletRepository) GetWalletBalance(walletID uuid.UUID, assetSymbol string) (*domain.WalletBalance, error) {
	var balance domain.WalletBalance
	if err := r.db.Where("wallet_id = ? AND asset_symbol = ?", walletID, assetSymbol).First(&balance).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &balance, nil
}

func (r *walletRepository) GetBalancesByWalletID(walletID uuid.UUID) ([]domain.WalletBalance, error) {
	var balances []domain.WalletBalance
	if err := r.db.Where("wallet_id = ?", walletID).Find(&balances).Error; err != nil {
		return nil, err
	}
	return balances, nil
}

// GetOrCreateBalance retrieves a balance row, or creates one with a zero balance if it doesn't exist.
// This is used when a new asset type is credited for the first time (e.g., first crypto deposit).
func (r *walletRepository) GetOrCreateBalance(walletID uuid.UUID, assetSymbol string) (*domain.WalletBalance, error) {
	balance := domain.WalletBalance{
		WalletID:    walletID,
		AssetSymbol: assetSymbol,
		Balance:     decimal.Zero,
		LastUpdated: time.Now(),
	}

	// Use GORM's FirstOrCreate to atomically fetch or create the balance row.
	result := r.db.Where(domain.WalletBalance{WalletID: walletID, AssetSymbol: assetSymbol}).
		Attrs(domain.WalletBalance{Balance: decimal.Zero, LastUpdated: time.Now()}).
		FirstOrCreate(&balance)

	if result.Error != nil {
		return nil, fmt.Errorf("failed to get or create balance: %w", result.Error)
	}

	return &balance, nil
}

// lockBalanceForUpdate fetches and locks a balance row within a GORM transaction.
// Returns an error wrapping domain.ErrNotFound if the row does not exist.
func lockBalanceForUpdate(tx *gorm.DB, walletID uuid.UUID, assetSymbol string) (decimal.Decimal, error) {
	var balance domain.WalletBalance
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("wallet_id = ? AND asset_symbol = ?", walletID, assetSymbol).
		First(&balance).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return decimal.Zero, fmt.Errorf("%w: balance not found for asset %s", domain.ErrNotFound, assetSymbol)
		}
		return decimal.Zero, fmt.Errorf("failed to lock balance: %w", err)
	}
	return balance.Balance, nil
}
