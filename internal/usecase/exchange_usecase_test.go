package usecase

import (
	"errors"
	"testing"
	"time"

	"github.com/bntngridp/ledger-backend/internal/domain"
	"github.com/bntngridp/ledger-backend/pkg/price"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGetExchangeRate_Success(t *testing.T) {
	// Initialize a real PriceCache with local settings (using Binance mock or real API URL with fallback)
	pc := price.NewPriceCache("https://api.binance.com/api/v3", decimal.NewFromInt(16200))
	uc := NewExchangeUsecase(nil, nil, pc, decimal.NewFromFloat(0.005))

	resp, err := uc.GetExchangeRate("USDT_IDR")
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "USDT_IDR", resp.Pair)
	// The rate should be around 16200 (since USDTUSDT is ~1.0)
	assert.True(t, resp.Rate.IsPositive())
}

func TestSwap_IDRToUSDT_Success(t *testing.T) {
	mockWalletRepo := new(MockWalletRepository)
	mockTxRepo := new(MockTransactionRepository)

	// Local PriceCache with fallback rate 16200
	pc := price.NewPriceCache("https://api.binance.com/api/v3", decimal.NewFromInt(16200))
	uc := NewExchangeUsecase(mockWalletRepo, mockTxRepo, pc, decimal.NewFromFloat(0.005)) // 0.5% fee

	userID := uuid.New()
	walletID := uuid.New()
	wallet := &domain.Wallet{
		WalletID: walletID,
		UserID:   userID,
	}

	balanceIDR := &domain.WalletBalance{
		WalletID:    walletID,
		AssetSymbol: "IDR",
		Balance:     decimal.NewFromInt(324000), // Rp 324.000
	}

	expectedTx := &domain.Transaction{
		TransactionID: uuid.New(),
		SourceWalletID: &walletID,
		Amount:        decimal.NewFromInt(162000),
		Type:          "swap",
		Status:        "success",
		CreatedAt:     time.Now(),
	}

	mockWalletRepo.On("GetWalletByUserID", userID).Return(wallet, nil)
	mockWalletRepo.On("GetWalletBalance", walletID, "IDR").Return(balanceIDR, nil)

	// Calculations for Swap:
	// fromAmount = 162000 IDR
	// grossToAmount = 162000 / 16200 = 10 USDT
	// fee = 10 * 0.5% = 0.05 USDT
	// netToAmount = 9.95 USDT
	mockTxRepo.On("ExecuteSwapTx", walletID, "IDR", "USDT",
		mock.MatchedBy(func(d decimal.Decimal) bool { return d.Equal(decimal.NewFromInt(162000)) }),
		mock.MatchedBy(func(d decimal.Decimal) bool { return d.Equal(decimal.NewFromFloat(9.95)) }),
		mock.Anything,
		mock.MatchedBy(func(d decimal.Decimal) bool { return d.Equal(decimal.NewFromFloat(0.05)) }),
	).Return(expectedTx, nil)

	req := domain.SwapRequest{
		FromAsset: "IDR",
		ToAsset:   "USDT",
		Amount:    decimal.NewFromInt(162000),
	}

	resp, err := uc.Swap(userID, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "IDR", resp.FromAsset)
	assert.Equal(t, "USDT", resp.ToAsset)
	assert.True(t, decimal.NewFromInt(162000).Equal(resp.FromAmount))
	assert.True(t, decimal.NewFromFloat(9.95).Equal(resp.ToAmount))
	assert.True(t, decimal.NewFromFloat(0.05).Equal(resp.FeeCharged))

	mockWalletRepo.AssertExpectations(t)
	mockTxRepo.AssertExpectations(t)
}

func TestSwap_USDTToIDR_Success(t *testing.T) {
	mockWalletRepo := new(MockWalletRepository)
	mockTxRepo := new(MockTransactionRepository)

	pc := price.NewPriceCache("https://api.binance.com/api/v3", decimal.NewFromInt(16200))
	uc := NewExchangeUsecase(mockWalletRepo, mockTxRepo, pc, decimal.NewFromFloat(0.005)) // 0.5% fee

	userID := uuid.New()
	walletID := uuid.New()
	wallet := &domain.Wallet{
		WalletID: walletID,
		UserID:   userID,
	}

	balanceUSDT := &domain.WalletBalance{
		WalletID:    walletID,
		AssetSymbol: "USDT",
		Balance:     decimal.NewFromInt(20), // 20 USDT
	}

	expectedTx := &domain.Transaction{
		TransactionID: uuid.New(),
		SourceWalletID: &walletID,
		Amount:        decimal.NewFromInt(10),
		Type:          "swap",
		Status:        "success",
		CreatedAt:     time.Now(),
	}

	mockWalletRepo.On("GetWalletByUserID", userID).Return(wallet, nil)
	mockWalletRepo.On("GetWalletBalance", walletID, "USDT").Return(balanceUSDT, nil)

	// Calculations for Swap:
	// fromAmount = 10 USDT
	// grossToAmount = 10 * 16200 = 162000 IDR
	// fee = 162000 * 0.5% = 810 IDR
	// netToAmount = 161190 IDR
	mockTxRepo.On("ExecuteSwapTx", walletID, "USDT", "IDR",
		mock.MatchedBy(func(d decimal.Decimal) bool { return d.Equal(decimal.NewFromInt(10)) }),
		mock.MatchedBy(func(d decimal.Decimal) bool { return d.Equal(decimal.NewFromInt(161190)) }),
		mock.Anything,
		mock.MatchedBy(func(d decimal.Decimal) bool { return d.Equal(decimal.NewFromInt(810)) }),
	).Return(expectedTx, nil)

	req := domain.SwapRequest{
		FromAsset: "USDT",
		ToAsset:   "IDR",
		Amount:    decimal.NewFromInt(10),
	}

	resp, err := uc.Swap(userID, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "USDT", resp.FromAsset)
	assert.Equal(t, "IDR", resp.ToAsset)
	assert.True(t, decimal.NewFromInt(10).Equal(resp.FromAmount))
	assert.True(t, decimal.NewFromInt(161190).Equal(resp.ToAmount))
	assert.True(t, decimal.NewFromInt(810).Equal(resp.FeeCharged))

	mockWalletRepo.AssertExpectations(t)
	mockTxRepo.AssertExpectations(t)
}

func TestSwap_InsufficientBalance(t *testing.T) {
	mockWalletRepo := new(MockWalletRepository)
	mockTxRepo := new(MockTransactionRepository)

	pc := price.NewPriceCache("https://api.binance.com/api/v3", decimal.NewFromInt(16200))
	uc := NewExchangeUsecase(mockWalletRepo, mockTxRepo, pc, decimal.NewFromFloat(0.005))

	userID := uuid.New()
	walletID := uuid.New()
	wallet := &domain.Wallet{
		WalletID: walletID,
		UserID:   userID,
	}

	balanceUSDT := &domain.WalletBalance{
		WalletID:    walletID,
		AssetSymbol: "USDT",
		Balance:     decimal.NewFromInt(5), // Only 5 USDT
	}

	mockWalletRepo.On("GetWalletByUserID", userID).Return(wallet, nil)
	mockWalletRepo.On("GetWalletBalance", walletID, "USDT").Return(balanceUSDT, nil)

	req := domain.SwapRequest{
		FromAsset: "USDT",
		ToAsset:   "IDR",
		Amount:    decimal.NewFromInt(10), // Needs 10 USDT
	}

	resp, err := uc.Swap(userID, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.True(t, errors.Is(err, domain.ErrInsufficientBalance))
}

func TestSwap_SameAsset(t *testing.T) {
	uc := NewExchangeUsecase(nil, nil, nil, decimal.NewFromFloat(0.005))
	userID := uuid.New()

	req := domain.SwapRequest{
		FromAsset: "USDT",
		ToAsset:   "USDT",
		Amount:    decimal.NewFromInt(10),
	}

	resp, err := uc.Swap(userID, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.True(t, errors.Is(err, domain.ErrSameAssetSwap))
}
