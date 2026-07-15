package usecase

import (
	"testing"

	"github.com/bntngridp/ledger-backend/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestProcessMidtransNotification_Success(t *testing.T) {
	mockTxRepo := new(MockTransactionRepository)
	mockMidtrans := new(MockMidtransClient)
	uc := NewWebhookUsecase(mockTxRepo, mockMidtrans)

	walletID := uuid.New()
	txID := uuid.New()
	orderID := "TOPUP-IDR-12345"

	payload := map[string]interface{}{
		"order_id":           orderID,
		"status_code":        "200",
		"gross_amount":       "100000.00",
		"signature_key":      "valid_signature",
		"transaction_status": "settlement",
	}

	txRecord := &domain.Transaction{
		TransactionID:       txID,
		DestinationWalletID: &walletID,
		Status:              "pending",
		AssetSymbol:         "IDR",
		Amount:              decimal.NewFromInt(100000),
	}

	mockMidtrans.On("VerifySignature", orderID, "200", "100000.00", "valid_signature").Return(true)
	mockTxRepo.On("GetTransactionByOrderID", orderID).Return(txRecord, nil)
	mockTxRepo.On("SettleTopUpTx", txID, walletID, mock.MatchedBy(func(d decimal.Decimal) bool {
		return d.Equal(decimal.NewFromFloat(100000.00))
	})).Return(nil)

	err := uc.ProcessMidtransNotification(payload)

	assert.NoError(t, err)
	mockMidtrans.AssertExpectations(t)
	mockTxRepo.AssertExpectations(t)
}

func TestProcessMidtransNotification_InvalidSignature(t *testing.T) {
	mockTxRepo := new(MockTransactionRepository)
	mockMidtrans := new(MockMidtransClient)
	uc := NewWebhookUsecase(mockTxRepo, mockMidtrans)

	orderID := "TOPUP-IDR-12345"
	payload := map[string]interface{}{
		"order_id":      orderID,
		"status_code":   "200",
		"gross_amount":  "100000.00",
		"signature_key": "invalid_signature",
	}

	mockMidtrans.On("VerifySignature", orderID, "200", "100000.00", "invalid_signature").Return(false)

	err := uc.ProcessMidtransNotification(payload)

	assert.Error(t, err)
	assert.Equal(t, "invalid signature key", err.Error())
	mockMidtrans.AssertExpectations(t)
	mockTxRepo.AssertNotCalled(t, "GetTransactionByOrderID")
}

func TestProcessMidtransNotification_FailedPayment(t *testing.T) {
	mockTxRepo := new(MockTransactionRepository)
	mockMidtrans := new(MockMidtransClient)
	uc := NewWebhookUsecase(mockTxRepo, mockMidtrans)

	walletID := uuid.New()
	txID := uuid.New()
	orderID := "TOPUP-IDR-12345"

	payload := map[string]interface{}{
		"order_id":           orderID,
		"status_code":        "201",
		"gross_amount":       "100000.00",
		"signature_key":      "valid_signature",
		"transaction_status": "expire",
	}

	txRecord := &domain.Transaction{
		TransactionID:       txID,
		DestinationWalletID: &walletID,
		Status:              "pending",
		AssetSymbol:         "IDR",
		Amount:              decimal.NewFromInt(100000),
	}

	mockMidtrans.On("VerifySignature", orderID, "201", "100000.00", "valid_signature").Return(true)
	mockTxRepo.On("GetTransactionByOrderID", orderID).Return(txRecord, nil)
	mockTxRepo.On("UpdateTransactionStatus", txID, "failed", "Payment expire").Return(nil)

	err := uc.ProcessMidtransNotification(payload)

	assert.NoError(t, err)
	mockMidtrans.AssertExpectations(t)
	mockTxRepo.AssertExpectations(t)
}

func TestProcessMidtransNotification_TxAlreadyProcessed(t *testing.T) {
	mockTxRepo := new(MockTransactionRepository)
	mockMidtrans := new(MockMidtransClient)
	uc := NewWebhookUsecase(mockTxRepo, mockMidtrans)

	orderID := "TOPUP-IDR-12345"

	payload := map[string]interface{}{
		"order_id":           orderID,
		"status_code":        "200",
		"gross_amount":       "100000.00",
		"signature_key":      "valid_signature",
		"transaction_status": "settlement",
	}

	txRecord := &domain.Transaction{
		TransactionID: uuid.New(),
		Status:        "success", // Already successful!
	}

	mockMidtrans.On("VerifySignature", orderID, "200", "100000.00", "valid_signature").Return(true)
	mockTxRepo.On("GetTransactionByOrderID", orderID).Return(txRecord, nil)

	err := uc.ProcessMidtransNotification(payload)

	assert.NoError(t, err) // Idempotency check: should return nil error
	mockTxRepo.AssertNotCalled(t, "SettleTopUpTx")
	mockTxRepo.AssertNotCalled(t, "UpdateTransactionStatus")
}

func TestProcessMidtransNotification_TxNotFound(t *testing.T) {
	mockTxRepo := new(MockTransactionRepository)
	mockMidtrans := new(MockMidtransClient)
	uc := NewWebhookUsecase(mockTxRepo, mockMidtrans)

	orderID := "TOPUP-IDR-12345"

	payload := map[string]interface{}{
		"order_id":           orderID,
		"status_code":        "200",
		"gross_amount":       "100000.00",
		"signature_key":      "valid_signature",
		"transaction_status": "settlement",
	}

	mockMidtrans.On("VerifySignature", orderID, "200", "100000.00", "valid_signature").Return(true)
	mockTxRepo.On("GetTransactionByOrderID", orderID).Return(nil, nil)

	err := uc.ProcessMidtransNotification(payload)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "transaction not found")
}

func TestProcessIrisNotification_Success(t *testing.T) {
	mockTxRepo := new(MockTransactionRepository)
	mockMidtrans := new(MockMidtransClient)
	uc := NewWebhookUsecase(mockTxRepo, mockMidtrans)

	txUUID := uuid.New()
	payload := []domain.IrisCallbackItem{
		{
			PayoutID:    123456,
			ReferenceNo: txUUID.String(),
			Amount:      "50000.00",
			Status:      "completed",
		},
	}

	mockTxRepo.On("UpdateTransactionStatus", txUUID, "success", "").Return(nil)

	err := uc.ProcessIrisNotification(payload)

	assert.NoError(t, err)
	mockTxRepo.AssertExpectations(t)
}

func TestProcessIrisNotification_Failed(t *testing.T) {
	mockTxRepo := new(MockTransactionRepository)
	mockMidtrans := new(MockMidtransClient)
	uc := NewWebhookUsecase(mockTxRepo, mockMidtrans)

	txUUID := uuid.New()
	reason := "Insufficient balance in partner account"
	payload := []domain.IrisCallbackItem{
		{
			PayoutID:     123456,
			ReferenceNo:  txUUID.String(),
			Amount:       "50000.00",
			Status:       "failed",
			ErrorMessage: &reason,
		},
	}

	mockTxRepo.On("RejectWithdrawFiatTx", txUUID, reason).Return(nil)

	err := uc.ProcessIrisNotification(payload)

	assert.NoError(t, err)
	mockTxRepo.AssertExpectations(t)
}

