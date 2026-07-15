package usecase

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/bntngridp/ledger-backend/internal/domain"
	"github.com/bntngridp/ledger-backend/pkg/midtrans"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type WebhookUsecase interface {
	ProcessMidtransNotification(payload map[string]interface{}) error
	ProcessIrisNotification(payload []domain.IrisCallbackItem) error
}

type webhookUsecase struct {
	txRepo         domain.TransactionRepository
	midtransClient midtrans.Client
}

func NewWebhookUsecase(txRepo domain.TransactionRepository, midtransClient midtrans.Client) WebhookUsecase {
	return &webhookUsecase{
		txRepo:         txRepo,
		midtransClient: midtransClient,
	}
}

func (uc *webhookUsecase) ProcessMidtransNotification(payload map[string]interface{}) error {
	orderID, ok := payload["order_id"].(string)
	if !ok || orderID == "" {
		return errors.New("missing or invalid order_id")
	}

	statusCode, ok := payload["status_code"].(string)
	if !ok || statusCode == "" {
		return errors.New("missing or invalid status_code")
	}

	grossAmount, ok := payload["gross_amount"].(string)
	if !ok || grossAmount == "" {
		return errors.New("missing or invalid gross_amount")
	}

	signatureKey, ok := payload["signature_key"].(string)
	if !ok || signatureKey == "" {
		return errors.New("missing or invalid signature_key")
	}

	// 1. Verify signature
	if !uc.midtransClient.VerifySignature(orderID, statusCode, grossAmount, signatureKey) {
		log.Printf("invalid signature key received for order: %s", orderID)
		return errors.New("invalid signature key")
	}

	// 2. Fetch transaction
	txRecord, err := uc.txRepo.GetTransactionByOrderID(orderID)
	if err != nil {
		return fmt.Errorf("failed to fetch transaction: %w", err)
	}
	if txRecord == nil {
		return fmt.Errorf("transaction not found for order: %s", orderID)
	}

	// 3. Ensure transaction is pending
	if txRecord.Status != "pending" {
		log.Printf("transaction %s has already been processed with status: %s", txRecord.TransactionID, txRecord.Status)
		return nil // Return nil so we don't cause duplicate request errors to Midtrans
	}

	// 4. Parse amount
	amt, err := decimal.NewFromString(grossAmount)
	if err != nil {
		return fmt.Errorf("failed to parse gross_amount: %w", err)
	}

	txStatus, _ := payload["transaction_status"].(string)
	fraudStatus, _ := payload["fraud_status"].(string)

	log.Printf("processing midtrans transaction %s: status=%s, fraud=%s", orderID, txStatus, fraudStatus)

	// 5. Handle payment status
	switch txStatus {
	case "capture":
		if fraudStatus == "accept" {
			return uc.settleTransaction(txRecord, amt)
		}
		return uc.failTransaction(txRecord, "Captured but fraud status was "+fraudStatus)
	case "settlement":
		return uc.settleTransaction(txRecord, amt)
	case "deny", "cancel", "expire":
		return uc.failTransaction(txRecord, "Payment "+txStatus)
	default:
		log.Printf("unhandled transaction status: %s", txStatus)
		return nil
	}
}

func (uc *webhookUsecase) settleTransaction(txRecord *domain.Transaction, amount decimal.Decimal) error {
	if txRecord.DestinationWalletID == nil {
		return errors.New("destination wallet is nil")
	}
	log.Printf("settling transaction %s for wallet %s with amount %s", txRecord.TransactionID, txRecord.DestinationWalletID.String(), amount.String())
	return uc.txRepo.SettleTopUpTx(txRecord.TransactionID, *txRecord.DestinationWalletID, amount)
}

func (uc *webhookUsecase) failTransaction(txRecord *domain.Transaction, reason string) error {
	log.Printf("failing transaction %s. Reason: %s", txRecord.TransactionID, reason)
	return uc.txRepo.UpdateTransactionStatus(txRecord.TransactionID, "failed", reason)
}

func (uc *webhookUsecase) ProcessIrisNotification(payload []domain.IrisCallbackItem) error {
	for _, item := range payload {
		txUUID, err := uuid.Parse(item.ReferenceNo)
		if err != nil {
			log.Printf("[Iris Webhook] invalid reference_no UUID: %s", item.ReferenceNo)
			continue
		}

		status := strings.ToLower(item.Status)
		log.Printf("[Iris Webhook] processing item: ref=%s, status=%s", item.ReferenceNo, status)

		if status == "completed" {
			err = uc.txRepo.UpdateTransactionStatus(txUUID, "success", "")
		} else if status == "failed" {
			errMsg := "Iris callback failure"
			if item.ErrorMessage != nil {
				errMsg = *item.ErrorMessage
			}
			err = uc.txRepo.RejectWithdrawFiatTx(txUUID, errMsg)
		} else {
			log.Printf("[Iris Webhook] unhandled status: %s for reference_no: %s", item.Status, item.ReferenceNo)
			continue
		}

		if err != nil {
			log.Printf("[Iris Webhook] failed to process item %s: %v", item.ReferenceNo, err)
			return err
		}
	}
	return nil
}

