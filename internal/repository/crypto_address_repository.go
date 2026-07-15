package repository

import (
	"errors"
	"fmt"

	"github.com/bntngridp/ledger-backend/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type cryptoAddressRepository struct {
	db *gorm.DB
}

func NewCryptoAddressRepository(db *gorm.DB) domain.CryptoAddressRepository {
	return &cryptoAddressRepository{db: db}
}

func (r *cryptoAddressRepository) GetAddressByWalletID(walletID uuid.UUID, network, assetSymbol string) (*domain.CryptoAddress, error) {
	var cryptoAddr domain.CryptoAddress
	if err := r.db.Where("wallet_id = ? AND network = ? AND asset_symbol = ?", walletID, network, assetSymbol).First(&cryptoAddr).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get crypto address: %w", err)
	}
	return &cryptoAddr, nil
}

func (r *cryptoAddressRepository) GetAddressByValue(address string) (*domain.CryptoAddress, error) {
	var cryptoAddr domain.CryptoAddress
	if err := r.db.Where("address = ?", address).First(&cryptoAddr).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get crypto address by value: %w", err)
	}
	return &cryptoAddr, nil
}

func (r *cryptoAddressRepository) CreateAddress(cryptoAddr *domain.CryptoAddress) error {
	if err := r.db.Create(cryptoAddr).Error; err != nil {
		return fmt.Errorf("failed to create crypto address: %w", err)
	}
	return nil
}

// GetAllAddresses returns all deposit addresses for a given network.
// Used by the On-Chain Listener to build its watch list on startup.
func (r *cryptoAddressRepository) GetAllAddresses(network string) ([]domain.CryptoAddress, error) {
	var addresses []domain.CryptoAddress
	if err := r.db.Where("network = ?", network).Find(&addresses).Error; err != nil {
		return nil, fmt.Errorf("failed to get all crypto addresses: %w", err)
	}
	return addresses, nil
}
