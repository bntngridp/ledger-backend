package usecase

import (
	"errors"
	"testing"

	"github.com/bntngridp/ledger-backend/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Base64 encoding of 32-byte key: "12345678901234567890123456789012"
const testEncryptionKey = "MTIzNDU2Nzg5MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTI="

func TestGetOrCreateDepositAddress_NewAddress(t *testing.T) {
	mockWalletRepo := new(MockWalletRepository)
	mockTxRepo := new(MockTransactionRepository)
	mockCryptoAddrRepo := new(MockCryptoAddressRepository)

	cfg := CryptoUsecaseConfig{
		WalletRepo:          mockWalletRepo,
		TxRepo:              mockTxRepo,
		CryptoAddrRepo:      mockCryptoAddrRepo,
		EncryptionKeyBase64: testEncryptionKey,
		AlchemyClient:       nil,
		ContractAddrs:       nil,
		Listener:            nil,
	}

	uc, err := NewCryptoUsecase(cfg)
	assert.NoError(t, err)

	userID := uuid.New()
	walletID := uuid.New()
	wallet := &domain.Wallet{
		WalletID: walletID,
		UserID:   userID,
	}

	mockWalletRepo.On("GetWalletByUserID", userID).Return(wallet, nil)
	// Return nil, nil to simulate no existing address
	mockCryptoAddrRepo.On("GetAddressByWalletID", walletID, "polygon_amoy", "USDT").Return(nil, nil)
	// Expect address creation
	mockCryptoAddrRepo.On("CreateAddress", mock.AnythingOfType("*domain.CryptoAddress")).Return(nil)

	resp, err := uc.GetOrCreateDepositAddress(userID, "polygon_amoy", "USDT")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "polygon_amoy", resp.Network)
	assert.Equal(t, "USDT", resp.AssetSymbol)
	assert.NotEmpty(t, resp.Address)
	assert.Contains(t, resp.Address, "0x")

	mockWalletRepo.AssertExpectations(t)
	mockCryptoAddrRepo.AssertExpectations(t)
}

func TestGetOrCreateDepositAddress_ExistingAddress(t *testing.T) {
	mockWalletRepo := new(MockWalletRepository)
	mockTxRepo := new(MockTransactionRepository)
	mockCryptoAddrRepo := new(MockCryptoAddressRepository)

	cfg := CryptoUsecaseConfig{
		WalletRepo:          mockWalletRepo,
		TxRepo:              mockTxRepo,
		CryptoAddrRepo:      mockCryptoAddrRepo,
		EncryptionKeyBase64: testEncryptionKey,
		AlchemyClient:       nil,
		ContractAddrs:       nil,
		Listener:            nil,
	}

	uc, err := NewCryptoUsecase(cfg)
	assert.NoError(t, err)

	userID := uuid.New()
	walletID := uuid.New()
	wallet := &domain.Wallet{
		WalletID: walletID,
		UserID:   userID,
	}

	existingAddress := &domain.CryptoAddress{
		WalletID:      walletID,
		Network:       "polygon_amoy",
		AssetSymbol:   "USDT",
		Address:       "0x1234567890123456789012345678901234567890",
		EncPrivateKey: "someencryptedkey",
	}

	mockWalletRepo.On("GetWalletByUserID", userID).Return(wallet, nil)
	mockCryptoAddrRepo.On("GetAddressByWalletID", walletID, "polygon_amoy", "USDT").Return(existingAddress, nil)

	resp, err := uc.GetOrCreateDepositAddress(userID, "polygon_amoy", "USDT")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, existingAddress.Address, resp.Address)
	// CreateAddress should NOT be called
	mockCryptoAddrRepo.AssertNotCalled(t, "CreateAddress")
}

func TestWithdrawCrypto_Success(t *testing.T) {
	mockWalletRepo := new(MockWalletRepository)
	mockTxRepo := new(MockTransactionRepository)
	mockCryptoAddrRepo := new(MockCryptoAddressRepository)

	cfg := CryptoUsecaseConfig{
		WalletRepo:          mockWalletRepo,
		TxRepo:              mockTxRepo,
		CryptoAddrRepo:      mockCryptoAddrRepo,
		EncryptionKeyBase64: testEncryptionKey,
		AlchemyClient:       nil,
		ContractAddrs:       nil,
		Listener:            nil,
	}

	uc, err := NewCryptoUsecase(cfg)
	assert.NoError(t, err)

	userID := uuid.New()
	walletID := uuid.New()
	wallet := &domain.Wallet{
		WalletID: walletID,
		UserID:   userID,
	}

	expectedTx := &domain.Transaction{
		TransactionID:  uuid.New(),
		SourceWalletID: &walletID,
		Amount:         decimal.NewFromFloat(5.5),
		Type:           "crypto_withdrawal",
		Status:         "pending",
	}

	mockWalletRepo.On("GetWalletByUserID", userID).Return(wallet, nil)
	mockTxRepo.On("CreatePendingCryptoWithdrawTx", walletID, decimal.NewFromFloat(5.5), "USDT", "0x1234567890123456789012345678901234567890", mock.Anything).
		Return(expectedTx, nil)

	req := domain.CryptoWithdrawRequest{
		AssetSymbol: "USDT",
		Network:     "polygon_amoy",
		ToAddress:   "0x1234567890123456789012345678901234567890",
		Amount:      decimal.NewFromFloat(5.5),
	}

	resp, err := uc.WithdrawCrypto(userID, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, expectedTx.TransactionID.String(), resp.TransactionID)
	assert.Equal(t, "USDT", resp.AssetSymbol)
	assert.Equal(t, "pending", resp.Status)

	mockWalletRepo.AssertExpectations(t)
	mockTxRepo.AssertExpectations(t)
}

func TestWithdrawCrypto_InvalidAddress(t *testing.T) {
	mockWalletRepo := new(MockWalletRepository)
	mockTxRepo := new(MockTransactionRepository)
	mockCryptoAddrRepo := new(MockCryptoAddressRepository)

	cfg := CryptoUsecaseConfig{
		WalletRepo:          mockWalletRepo,
		TxRepo:              mockTxRepo,
		CryptoAddrRepo:      mockCryptoAddrRepo,
		EncryptionKeyBase64: testEncryptionKey,
		AlchemyClient:       nil,
		ContractAddrs:       nil,
		Listener:            nil,
	}

	uc, err := NewCryptoUsecase(cfg)
	assert.NoError(t, err)

	userID := uuid.New()

	req := domain.CryptoWithdrawRequest{
		AssetSymbol: "USDT",
		Network:     "polygon_amoy",
		ToAddress:   "invalid-eth-address", // Invalid address
		Amount:      decimal.NewFromFloat(5.5),
	}

	resp, err := uc.WithdrawCrypto(userID, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.True(t, errors.Is(err, domain.ErrInvalidAddress))
}
