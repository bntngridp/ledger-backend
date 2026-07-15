package usecase

import (
	"errors"
	"testing"
	"time"

	"github.com/bntngridp/ledger-backend/internal/domain"
	"github.com/bntngridp/ledger-backend/pkg/price"
	"github.com/google/uuid"
	"github.com/midtrans/midtrans-go/snap"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestTopUp_Success(t *testing.T) {
	mockWalletRepo := new(MockWalletRepository)
	mockTxRepo := new(MockTransactionRepository)
	mockMidtrans := new(MockMidtransClient)
	pc := price.NewPriceCache("https://api.binance.com/api/v3", decimal.NewFromInt(16200))
	uc := NewWalletUsecase(mockWalletRepo, mockTxRepo, mockMidtrans, pc)

	userID := uuid.New()
	walletID := uuid.New()
	wallet := &domain.Wallet{
		WalletID: walletID,
		UserID:   userID,
		User: &domain.User{
			Email:    "test@example.com",
			Username: "testuser",
		},
	}

	expectedTxID := uuid.New()
	expectedTx := &domain.Transaction{
		TransactionID:       expectedTxID,
		DestinationWalletID: &walletID,
		Amount:              decimal.NewFromInt(100000),
		Type:                "topup",
		Status:              "pending",
	}

	mockWalletRepo.On("GetWalletByUserID", userID).Return(wallet, nil)
	mockTxRepo.On("CreatePendingTopUpTx", walletID, decimal.NewFromInt(100000), "IDR", mock.Anything, "topup awal").
		Return(expectedTx, nil)

	snapResp := &snap.Response{
		Token:       "snap-token-123",
		RedirectURL: "https://redirect-url.com",
	}
	mockMidtrans.On("CreateSnapTransaction", mock.Anything, decimal.NewFromInt(100000), "test@example.com", "testuser").
		Return(snapResp, nil)

	resp, err := uc.TopUp(userID, decimal.NewFromInt(100000), "IDR", "topup awal")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, expectedTxID.String(), resp.TransactionID)
	assert.Equal(t, walletID.String(), resp.WalletID)
	assert.True(t, decimal.NewFromInt(100000).Equal(resp.Amount))
	assert.Equal(t, "snap-token-123", resp.SnapToken)
	assert.Equal(t, "https://redirect-url.com", resp.RedirectURL)
	mockWalletRepo.AssertExpectations(t)
	mockTxRepo.AssertExpectations(t)
	mockMidtrans.AssertExpectations(t)
}

func TestTopUp_ZeroAmount(t *testing.T) {
	mockWalletRepo := new(MockWalletRepository)
	mockTxRepo := new(MockTransactionRepository)
	mockMidtrans := new(MockMidtransClient)
	pc := price.NewPriceCache("https://api.binance.com/api/v3", decimal.NewFromInt(16200))
	uc := NewWalletUsecase(mockWalletRepo, mockTxRepo, mockMidtrans, pc)

	userID := uuid.New()

	resp, err := uc.TopUp(userID, decimal.Zero, "IDR", "test")

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.True(t, errors.Is(err, domain.ErrInvalidInput))
	mockWalletRepo.AssertNotCalled(t, "GetWalletByUserID")
}

func TestGetTransactionHistory_Success(t *testing.T) {
	mockWalletRepo := new(MockWalletRepository)
	mockTxRepo := new(MockTransactionRepository)
	mockMidtrans := new(MockMidtransClient)
	pc := price.NewPriceCache("https://api.binance.com/api/v3", decimal.NewFromInt(16200))
	uc := NewWalletUsecase(mockWalletRepo, mockTxRepo, mockMidtrans, pc)

	userID := uuid.New()
	walletID := uuid.New()
	otherWalletID := uuid.New()
	wallet := &domain.Wallet{WalletID: walletID, UserID: userID}

	txTopUp := domain.Transaction{
		TransactionID:       uuid.New(),
		DestinationWalletID: &walletID,
		Amount:              decimal.NewFromInt(100000),
		Type:                "topup",
		Status:              "success",
		TransactionNotes:    "topup awal",
		CreatedAt:           time.Now(),
	}
	txTransferIn := domain.Transaction{
		TransactionID:       uuid.New(),
		SourceWalletID:      &otherWalletID,
		DestinationWalletID: &walletID,
		Amount:              decimal.NewFromInt(25005),
		Type:                "transfer_fiat",
		Status:              "success",
		TransactionNotes:    "from other user",
		CreatedAt:           time.Now().Add(-1 * time.Hour),
	}

	mockWalletRepo.On("GetWalletByUserID", userID).Return(wallet, nil)
	mockTxRepo.On("GetTransactionsByWalletID", walletID, 1, 20, "", "").
		Return([]domain.Transaction{txTopUp, txTransferIn}, int64(2), nil)

	historyResp, err := uc.GetTransactionHistory(userID, 1, 20, "", "")

	assert.NoError(t, err)
	assert.NotNil(t, historyResp)
	assert.Len(t, historyResp.Transactions, 2)
	assert.Equal(t, "topup", historyResp.Transactions[0].Type)
	assert.Equal(t, "transfer_fiat", historyResp.Transactions[1].Type)
	assert.Nil(t, historyResp.Transactions[0].SourceWalletID)
	assert.NotNil(t, historyResp.Transactions[1].SourceWalletID)
	assert.Equal(t, otherWalletID.String(), *historyResp.Transactions[1].SourceWalletID)
	assert.Equal(t, 1, historyResp.Meta.Page)
	assert.Equal(t, 20, historyResp.Meta.PerPage)
	assert.Equal(t, int64(2), historyResp.Meta.Total)
	assert.Equal(t, 1, historyResp.Meta.TotalPages)
	mockWalletRepo.AssertExpectations(t)
	mockTxRepo.AssertExpectations(t)
}

func TestGetTransactionHistory_EmptyList(t *testing.T) {
	mockWalletRepo := new(MockWalletRepository)
	mockTxRepo := new(MockTransactionRepository)
	mockMidtrans := new(MockMidtransClient)
	pc := price.NewPriceCache("https://api.binance.com/api/v3", decimal.NewFromInt(16200))
	uc := NewWalletUsecase(mockWalletRepo, mockTxRepo, mockMidtrans, pc)

	userID := uuid.New()
	walletID := uuid.New()
	wallet := &domain.Wallet{WalletID: walletID, UserID: userID}

	mockWalletRepo.On("GetWalletByUserID", userID).Return(wallet, nil)
	mockTxRepo.On("GetTransactionsByWalletID", walletID, 1, 20, "", "").
		Return([]domain.Transaction{}, int64(0), nil)

	historyResp, err := uc.GetTransactionHistory(userID, 1, 20, "", "")

	assert.NoError(t, err)
	assert.Len(t, historyResp.Transactions, 0)
	assert.Equal(t, int64(0), historyResp.Meta.Total)
}

func TestGetTransactionHistory_WalletNotFound(t *testing.T) {
	mockWalletRepo := new(MockWalletRepository)
	mockTxRepo := new(MockTransactionRepository)
	mockMidtrans := new(MockMidtransClient)
	pc := price.NewPriceCache("https://api.binance.com/api/v3", decimal.NewFromInt(16200))
	uc := NewWalletUsecase(mockWalletRepo, mockTxRepo, mockMidtrans, pc)

	userID := uuid.New()

	mockWalletRepo.On("GetWalletByUserID", userID).Return(nil, nil)

	historyResp, err := uc.GetTransactionHistory(userID, 1, 20, "", "")

	assert.Error(t, err)
	assert.Nil(t, historyResp)
	assert.True(t, errors.Is(err, domain.ErrNotFound))
	mockTxRepo.AssertNotCalled(t, "GetTransactionsByWalletID")
}

func TestGetTransactionHistory_GetWalletError(t *testing.T) {
	mockWalletRepo := new(MockWalletRepository)
	mockTxRepo := new(MockTransactionRepository)
	mockMidtrans := new(MockMidtransClient)
	pc := price.NewPriceCache("https://api.binance.com/api/v3", decimal.NewFromInt(16200))
	uc := NewWalletUsecase(mockWalletRepo, mockTxRepo, mockMidtrans, pc)

	userID := uuid.New()

	mockWalletRepo.On("GetWalletByUserID", userID).
		Return(nil, errors.New("db connection lost"))

	historyResp, err := uc.GetTransactionHistory(userID, 1, 20, "", "")

	assert.Error(t, err)
	assert.Nil(t, historyResp)
	assert.Contains(t, err.Error(), "failed to get wallet")
}

func TestGetTransactionHistory_RepoError(t *testing.T) {
	mockWalletRepo := new(MockWalletRepository)
	mockTxRepo := new(MockTransactionRepository)
	mockMidtrans := new(MockMidtransClient)
	pc := price.NewPriceCache("https://api.binance.com/api/v3", decimal.NewFromInt(16200))
	uc := NewWalletUsecase(mockWalletRepo, mockTxRepo, mockMidtrans, pc)

	userID := uuid.New()
	wallet := &domain.Wallet{WalletID: uuid.New(), UserID: userID}

	mockWalletRepo.On("GetWalletByUserID", userID).Return(wallet, nil)
	mockTxRepo.On("GetTransactionsByWalletID", wallet.WalletID, 1, 20, "", "").
		Return(nil, int64(0), errors.New("query timeout"))

	historyResp, err := uc.GetTransactionHistory(userID, 1, 20, "", "")

	assert.Error(t, err)
	assert.Nil(t, historyResp)
	assert.Equal(t, "query timeout", err.Error())
}

func TestGetDashboard_Success(t *testing.T) {
	mockWalletRepo := new(MockWalletRepository)
	mockTxRepo := new(MockTransactionRepository)
	mockMidtrans := new(MockMidtransClient)

	// Local PriceCache with local fallback (USDT=16200, USDC=16200)
	pc := price.NewPriceCache("https://api.binance.com/api/v3", decimal.NewFromInt(16200))
	uc := NewWalletUsecase(mockWalletRepo, mockTxRepo, mockMidtrans, pc)

	userID := uuid.New()
	walletID := uuid.New()
	wallet := &domain.Wallet{
		WalletID: walletID,
		UserID:   userID,
		Balances: []domain.WalletBalance{
			{AssetSymbol: "IDR", Balance: decimal.NewFromInt(50000)},
			{AssetSymbol: "USDT", Balance: decimal.NewFromInt(10)},
			{AssetSymbol: "USDC", Balance: decimal.NewFromInt(5)},
		},
	}

	mockWalletRepo.On("GetWalletByUserID", userID).Return(wallet, nil)

	dashboard, err := uc.GetDashboard(userID)

	assert.NoError(t, err)
	assert.NotNil(t, dashboard)
	assert.Equal(t, walletID.String(), dashboard.WalletID)
	assert.Len(t, dashboard.Balances, 3)

	// Since local priceCache fetches from Binance or falls back to 16200:
	// USDT_IDR: 16200, USDC_IDR: 16200
	// 50000*1 + 10*16200 + 5*16200 = 50000 + 162000 + 81000 = 293000
	// Let's assert it is positive and greater than 50000
	assert.True(t, dashboard.EstimatedTotalIDR.GreaterThan(decimal.NewFromInt(50000)))
	mockWalletRepo.AssertExpectations(t)
}
