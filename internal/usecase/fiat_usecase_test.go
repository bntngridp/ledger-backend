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

func TestWithdrawFiat_Success(t *testing.T) {
	mockWalletRepo := new(MockWalletRepository)
	mockTxRepo := new(MockTransactionRepository)

	// We pass nil for irisClient in test to test the DB-only path or we can mock it
	uc := NewFiatUsecase(mockWalletRepo, mockTxRepo, nil)

	userID := uuid.New()
	walletID := uuid.New()
	wallet := &domain.Wallet{
		WalletID: walletID,
		UserID:   userID,
	}

	balance := &domain.WalletBalance{
		WalletID:    walletID,
		AssetSymbol: "IDR",
		Balance:     decimal.NewFromInt(100000), // Rp 100.000
	}

	expectedTx := &domain.Transaction{
		TransactionID:  uuid.New(),
		SourceWalletID: &walletID,
		Amount:         decimal.NewFromInt(60000),
		Type:           "withdraw_fiat",
		Status:         "pending",
	}

	mockWalletRepo.On("GetWalletByUserID", userID).Return(wallet, nil)
	mockWalletRepo.On("GetWalletBalance", walletID, "IDR").Return(balance, nil)
	mockTxRepo.On("ExecuteWithdrawFiatTx", walletID, decimal.NewFromInt(60000), decimal.NewFromInt(2500), "IDR", mock.Anything).
		Return(expectedTx, nil)

	req := domain.WithdrawFiatRequest{
		Amount:        decimal.NewFromInt(60000),
		BankCode:      "bca",
		AccountNumber: "1234567890",
		AccountName:   "Bintang",
		Notes:         "tarik tunai",
	}

	resp, err := uc.WithdrawFiat(userID, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, expectedTx.TransactionID.String(), resp.TransactionID)
	assert.True(t, decimal.NewFromInt(60000).Equal(resp.Amount))
	assert.True(t, decimal.NewFromInt(2500).Equal(resp.AdminFee))
	assert.Equal(t, "pending", resp.Status)

	mockWalletRepo.AssertExpectations(t)
	mockTxRepo.AssertExpectations(t)
}

func TestWithdrawFiat_InsufficientBalance(t *testing.T) {
	mockWalletRepo := new(MockWalletRepository)
	mockTxRepo := new(MockTransactionRepository)
	uc := NewFiatUsecase(mockWalletRepo, mockTxRepo, nil)

	userID := uuid.New()
	walletID := uuid.New()
	wallet := &domain.Wallet{
		WalletID: walletID,
		UserID:   userID,
	}

	balance := &domain.WalletBalance{
		WalletID:    walletID,
		AssetSymbol: "IDR",
		Balance:     decimal.NewFromInt(50000), // Only Rp 50.000, withdrawal needs Rp 50.000 + Rp 2.500 fee
	}

	mockWalletRepo.On("GetWalletByUserID", userID).Return(wallet, nil)
	mockWalletRepo.On("GetWalletBalance", walletID, "IDR").Return(balance, nil)

	req := domain.WithdrawFiatRequest{
		Amount:        decimal.NewFromInt(50000),
		BankCode:      "bca",
		AccountNumber: "1234567890",
		AccountName:   "Bintang",
	}

	resp, err := uc.WithdrawFiat(userID, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.True(t, errors.Is(err, domain.ErrInsufficientBalance))
}

func TestWithdrawFiat_BelowMinimum(t *testing.T) {
	mockWalletRepo := new(MockWalletRepository)
	mockTxRepo := new(MockTransactionRepository)
	uc := NewFiatUsecase(mockWalletRepo, mockTxRepo, nil)

	userID := uuid.New()

	req := domain.WithdrawFiatRequest{
		Amount:        decimal.NewFromInt(10000), // Below Rp 50.000 minimum
		BankCode:      "bca",
		AccountNumber: "1234567890",
		AccountName:   "Bintang",
	}

	resp, err := uc.WithdrawFiat(userID, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "minimum withdrawal amount")
}
