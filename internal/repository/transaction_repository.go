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

type transactionRepository struct {
	db *gorm.DB
}

func NewTransactionRepository(db *gorm.DB) domain.TransactionRepository {
	return &transactionRepository{db: db}
}

// ============================================================
// Fiat / Transfer
// ============================================================

func (r *transactionRepository) ExecuteTransferTx(senderWalletID, recipientWalletID uuid.UUID, amount decimal.Decimal, assetSymbol, notes string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Lock rows in consistent order (lower UUID first) to prevent deadlocks.
		first, second := senderWalletID, recipientWalletID
		if senderWalletID.String() > recipientWalletID.String() {
			first, second = recipientWalletID, senderWalletID
		}

		// Lock both balance rows.
		var firstBal, secondBal domain.WalletBalance
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("wallet_id = ? AND asset_symbol = ?", first, assetSymbol).First(&firstBal).Error; err != nil {
			return fmt.Errorf("failed to lock first balance: %w", err)
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("wallet_id = ? AND asset_symbol = ?", second, assetSymbol).First(&secondBal).Error; err != nil {
			return fmt.Errorf("failed to lock second balance: %w", err)
		}

		var senderBalance, recipientBalance decimal.Decimal
		if senderWalletID == first {
			senderBalance = firstBal.Balance
			recipientBalance = secondBal.Balance
		} else {
			senderBalance = secondBal.Balance
			recipientBalance = firstBal.Balance
		}

		if senderBalance.LessThan(amount) {
			return domain.ErrInsufficientBalance
		}

		newSenderBalance := senderBalance.Sub(amount)
		newRecipientBalance := recipientBalance.Add(amount)

		if err := tx.Model(&domain.WalletBalance{}).Where("wallet_id = ? AND asset_symbol = ?", senderWalletID, assetSymbol).
			Updates(map[string]interface{}{"balance": newSenderBalance, "last_updated": time.Now()}).Error; err != nil {
			return fmt.Errorf("failed to debit sender: %w", err)
		}
		if err := tx.Model(&domain.WalletBalance{}).Where("wallet_id = ? AND asset_symbol = ?", recipientWalletID, assetSymbol).
			Updates(map[string]interface{}{"balance": newRecipientBalance, "last_updated": time.Now()}).Error; err != nil {
			return fmt.Errorf("failed to credit recipient: %w", err)
		}

		txn := domain.Transaction{
			TransactionID:       uuid.New(),
			SourceWalletID:      &senderWalletID,
			DestinationWalletID: &recipientWalletID,
			AssetSymbol:         assetSymbol,
			Amount:              amount,
			Type:                "transfer_fiat",
			Status:              "success",
			TransactionNotes:    notes,
		}
		return tx.Create(&txn).Error
	})
}

func (r *transactionRepository) ExecuteTopUpTx(walletID uuid.UUID, amount decimal.Decimal, assetSymbol, notes string) (*domain.Transaction, decimal.Decimal, error) {
	var newBalance decimal.Decimal
	var transaction domain.Transaction

	err := r.db.Transaction(func(tx *gorm.DB) error {
		current, err := lockBalanceForUpdate(tx, walletID, assetSymbol)
		if err != nil {
			return err
		}
		newBalance = current.Add(amount)

		if err := tx.Model(&domain.WalletBalance{}).Where("wallet_id = ? AND asset_symbol = ?", walletID, assetSymbol).
			Updates(map[string]interface{}{"balance": newBalance, "last_updated": time.Now()}).Error; err != nil {
			return fmt.Errorf("failed to update balance: %w", err)
		}

		transaction = domain.Transaction{
			TransactionID:       uuid.New(),
			DestinationWalletID: &walletID,
			AssetSymbol:         assetSymbol,
			Amount:              amount,
			Type:                "topup",
			Status:              "success",
			TransactionNotes:    notes,
		}
		return tx.Create(&transaction).Error
	})

	if err != nil {
		return nil, decimal.Zero, err
	}
	return &transaction, newBalance, nil
}

func (r *transactionRepository) ExecuteWithdrawFiatTx(walletID uuid.UUID, amount, adminFee decimal.Decimal, assetSymbol, notes string) (*domain.Transaction, error) {
	var transaction domain.Transaction
	totalDeduction := amount.Add(adminFee)

	err := r.db.Transaction(func(tx *gorm.DB) error {
		current, err := lockBalanceForUpdate(tx, walletID, assetSymbol)
		if err != nil {
			return err
		}
		if current.LessThan(totalDeduction) {
			return domain.ErrInsufficientBalance
		}

		newBalance := current.Sub(totalDeduction)
		if err := tx.Model(&domain.WalletBalance{}).Where("wallet_id = ? AND asset_symbol = ?", walletID, assetSymbol).
			Updates(map[string]interface{}{"balance": newBalance, "last_updated": time.Now()}).Error; err != nil {
			return fmt.Errorf("failed to debit balance: %w", err)
		}

		transaction = domain.Transaction{
			TransactionID:       uuid.New(),
			SourceWalletID:      &walletID,
			AssetSymbol:         assetSymbol,
			Amount:              amount,
			Type:                "withdraw_fiat",
			Status:              "pending",
			TransactionNotes:    notes,
			FeeCharged:          &adminFee,
		}
		return tx.Create(&transaction).Error
	})
	if err != nil {
		return nil, err
	}
	return &transaction, nil
}

// ============================================================
// Midtrans / Webhook
// ============================================================

func (r *transactionRepository) CreatePendingTopUpTx(walletID uuid.UUID, amount decimal.Decimal, assetSymbol, orderID, notes string) (*domain.Transaction, error) {
	transaction := domain.Transaction{
		TransactionID:       uuid.New(),
		DestinationWalletID: &walletID,
		AssetSymbol:         assetSymbol,
		Amount:              amount,
		Type:                "topup",
		Status:              "pending",
		MidtransOrderID:     &orderID,
		TransactionNotes:    notes,
	}
	if err := r.db.Create(&transaction).Error; err != nil {
		return nil, err
	}
	return &transaction, nil
}

func (r *transactionRepository) SettleTopUpTx(transactionID, walletID uuid.UUID, amount decimal.Decimal) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		current, err := lockBalanceForUpdate(tx, walletID, "IDR")
		if err != nil {
			return err
		}
		newBalance := current.Add(amount)

		if err := tx.Model(&domain.WalletBalance{}).Where("wallet_id = ? AND asset_symbol = 'IDR'", walletID).
			Updates(map[string]interface{}{"balance": newBalance, "last_updated": time.Now()}).Error; err != nil {
			return fmt.Errorf("failed to credit balance: %w", err)
		}
		return tx.Model(&domain.Transaction{}).Where("transaction_id = ?", transactionID).
			Update("status", "success").Error
	})
}

func (r *transactionRepository) GetTransactionByOrderID(orderID string) (*domain.Transaction, error) {
	var transaction domain.Transaction
	if err := r.db.Where("midtrans_order_id = ?", orderID).First(&transaction).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &transaction, nil
}

func (r *transactionRepository) UpdateTransactionStatus(txID uuid.UUID, status, notes string) error {
	updates := map[string]interface{}{"status": status}
	if notes != "" {
		updates["transaction_notes"] = notes
	}
	return r.db.Model(&domain.Transaction{}).Where("transaction_id = ?", txID).Updates(updates).Error
}

// ============================================================
// History with Pagination
// ============================================================

func (r *transactionRepository) GetTransactionsByWalletID(walletID uuid.UUID, page, perPage int, assetFilter, typeFilter string) ([]domain.Transaction, int64, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	query := r.db.Model(&domain.Transaction{}).
		Where("source_wallet_id = ? OR destination_wallet_id = ?", walletID, walletID)

	if assetFilter != "" {
		query = query.Where("asset_symbol = ?", assetFilter)
	}
	if typeFilter != "" {
		query = query.Where("type = ?", typeFilter)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count transactions: %w", err)
	}

	var transactions []domain.Transaction
	offset := (page - 1) * perPage
	if err := query.Order("created_at DESC").Offset(offset).Limit(perPage).Find(&transactions).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get transactions: %w", err)
	}

	return transactions, total, nil
}

// ============================================================
// Crypto Deposit (credited by On-Chain Listener)
// ============================================================

func (r *transactionRepository) CreditCryptoDeposit(walletID uuid.UUID, amount decimal.Decimal, assetSymbol, txHash, notes string) (*domain.Transaction, error) {
	var transaction domain.Transaction

	err := r.db.Transaction(func(tx *gorm.DB) error {
		// Idempotency check: reject if this tx_hash has already been processed.
		var existing domain.Transaction
		if err := tx.Where("tx_hash = ?", txHash).First(&existing).Error; err == nil {
			return domain.ErrDuplicateTransaction
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("failed to check idempotency: %w", err)
		}

		// Fetch and lock balance; auto-create if this is the first deposit.
		var current decimal.Decimal
		var balance domain.WalletBalance
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("wallet_id = ? AND asset_symbol = ?", walletID, assetSymbol).
			First(&balance).Error

		if errors.Is(err, gorm.ErrRecordNotFound) {
			// First credit for this asset — create the row.
			newBalance := domain.WalletBalance{
				WalletID:    walletID,
				AssetSymbol: assetSymbol,
				Balance:     amount,
				LastUpdated: time.Now(),
			}
			if err := tx.Create(&newBalance).Error; err != nil {
				return fmt.Errorf("failed to create balance row: %w", err)
			}
		} else if err != nil {
			return fmt.Errorf("failed to lock balance: %w", err)
		} else {
			current = balance.Balance
			newBalance := current.Add(amount)
			if err := tx.Model(&domain.WalletBalance{}).
				Where("wallet_id = ? AND asset_symbol = ?", walletID, assetSymbol).
				Updates(map[string]interface{}{"balance": newBalance, "last_updated": time.Now()}).Error; err != nil {
				return fmt.Errorf("failed to credit deposit: %w", err)
			}
		}

		transaction = domain.Transaction{
			TransactionID:       uuid.New(),
			DestinationWalletID: &walletID,
			AssetSymbol:         assetSymbol,
			Amount:              amount,
			Type:                "crypto_deposit",
			Status:              "success",
			TxHash:              &txHash,
			TransactionNotes:    notes,
		}
		return tx.Create(&transaction).Error
	})
	if err != nil {
		return nil, err
	}
	return &transaction, nil
}

// ============================================================
// Crypto Withdrawal
// ============================================================

func (r *transactionRepository) CreatePendingCryptoWithdrawTx(walletID uuid.UUID, amount decimal.Decimal, assetSymbol, toAddress, notes string) (*domain.Transaction, error) {
	var transaction domain.Transaction

	err := r.db.Transaction(func(tx *gorm.DB) error {
		current, err := lockBalanceForUpdate(tx, walletID, assetSymbol)
		if err != nil {
			return err
		}
		if current.LessThan(amount) {
			return domain.ErrInsufficientBalance
		}

		newBalance := current.Sub(amount)
		if err := tx.Model(&domain.WalletBalance{}).Where("wallet_id = ? AND asset_symbol = ?", walletID, assetSymbol).
			Updates(map[string]interface{}{"balance": newBalance, "last_updated": time.Now()}).Error; err != nil {
			return fmt.Errorf("failed to debit balance: %w", err)
		}

		transaction = domain.Transaction{
			TransactionID:    uuid.New(),
			SourceWalletID:   &walletID,
			AssetSymbol:      assetSymbol,
			Amount:           amount,
			Type:             "crypto_withdrawal",
			Status:           "pending",
			TransactionNotes: notes,
		}
		return tx.Create(&transaction).Error
	})
	if err != nil {
		return nil, err
	}
	return &transaction, nil
}

func (r *transactionRepository) UpdateCryptoWithdrawTx(txID uuid.UUID, txHash, status string) error {
	updates := map[string]interface{}{"status": status}
	if txHash != "" {
		updates["tx_hash"] = txHash
	}
	return r.db.Model(&domain.Transaction{}).Where("transaction_id = ?", txID).Updates(updates).Error
}

// ============================================================
// Swap
// ============================================================

func (r *transactionRepository) ExecuteSwapTx(walletID uuid.UUID, fromAsset, toAsset string, fromAmount, toAmount, rateUsed, feeCharged decimal.Decimal) (*domain.Transaction, error) {
	var transaction domain.Transaction

	err := r.db.Transaction(func(tx *gorm.DB) error {
		// Lock from-asset balance.
		fromBal, err := lockBalanceForUpdate(tx, walletID, fromAsset)
		if err != nil {
			return err
		}
		if fromBal.LessThan(fromAmount) {
			return domain.ErrInsufficientBalance
		}

		// Lock to-asset balance.
		toBal, err := lockBalanceForUpdate(tx, walletID, toAsset)
		if err != nil {
			return err
		}

		newFromBalance := fromBal.Sub(fromAmount)
		newToBalance := toBal.Add(toAmount)

		if err := tx.Model(&domain.WalletBalance{}).Where("wallet_id = ? AND asset_symbol = ?", walletID, fromAsset).
			Updates(map[string]interface{}{"balance": newFromBalance, "last_updated": time.Now()}).Error; err != nil {
			return fmt.Errorf("failed to debit from-asset: %w", err)
		}
		if err := tx.Model(&domain.WalletBalance{}).Where("wallet_id = ? AND asset_symbol = ?", walletID, toAsset).
			Updates(map[string]interface{}{"balance": newToBalance, "last_updated": time.Now()}).Error; err != nil {
			return fmt.Errorf("failed to credit to-asset: %w", err)
		}

		transaction = domain.Transaction{
			TransactionID:    uuid.New(),
			SourceWalletID:   &walletID,
			AssetSymbol:      fromAsset,
			Amount:           fromAmount,
			Type:             "swap",
			Status:           "success",
			RateUsed:         &rateUsed,
			FeeCharged:       &feeCharged,
			TransactionNotes: fmt.Sprintf("swap %s -> %s", fromAsset, toAsset),
		}
		return tx.Create(&transaction).Error
	})
	if err != nil {
		return nil, err
	}
	return &transaction, nil
}

func (r *transactionRepository) RejectWithdrawCryptoTx(txID uuid.UUID, reason string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var txn domain.Transaction
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("transaction_id = ?", txID).First(&txn).Error; err != nil {
			return fmt.Errorf("failed to lock transaction: %w", err)
		}

		if txn.Status != "pending" {
			// Transaction has already been finalized, do nothing (idempotency)
			return nil
		}

		if txn.SourceWalletID == nil {
			return errors.New("source wallet ID is nil")
		}

		// Lock wallet balance
		current, err := lockBalanceForUpdate(tx, *txn.SourceWalletID, txn.AssetSymbol)
		if err != nil {
			return err
		}

		// Return the funds back to user balance
		newBalance := current.Add(txn.Amount)
		if err := tx.Model(&domain.WalletBalance{}).Where("wallet_id = ? AND asset_symbol = ?", *txn.SourceWalletID, txn.AssetSymbol).
			Updates(map[string]interface{}{"balance": newBalance, "last_updated": time.Now()}).Error; err != nil {
			return fmt.Errorf("failed to refund balance: %w", err)
		}

		// Set status to failed
		notes := txn.TransactionNotes
		if reason != "" {
			notes = fmt.Sprintf("%s (Failed: %s)", notes, reason)
		}
		return tx.Model(&domain.Transaction{}).Where("transaction_id = ?", txID).
			Updates(map[string]interface{}{
				"status":            "failed",
				"transaction_notes": notes,
			}).Error
	})
}

func (r *transactionRepository) RejectWithdrawFiatTx(txID uuid.UUID, reason string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var txn domain.Transaction
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("transaction_id = ?", txID).First(&txn).Error; err != nil {
			return fmt.Errorf("failed to lock transaction: %w", err)
		}

		if txn.Status != "pending" {
			// Transaction has already been finalized, do nothing (idempotency)
			return nil
		}

		if txn.SourceWalletID == nil {
			return errors.New("source wallet ID is nil")
		}

		// Lock wallet balance
		current, err := lockBalanceForUpdate(tx, *txn.SourceWalletID, txn.AssetSymbol)
		if err != nil {
			return err
		}

		// Total refund is amount + admin fee
		adminFee := decimal.Zero
		if txn.FeeCharged != nil {
			adminFee = *txn.FeeCharged
		}
		totalRefund := txn.Amount.Add(adminFee)

		// Return funds
		newBalance := current.Add(totalRefund)
		if err := tx.Model(&domain.WalletBalance{}).Where("wallet_id = ? AND asset_symbol = ?", *txn.SourceWalletID, txn.AssetSymbol).
			Updates(map[string]interface{}{"balance": newBalance, "last_updated": time.Now()}).Error; err != nil {
			return fmt.Errorf("failed to refund balance: %w", err)
		}

		// Set status to failed
		notes := txn.TransactionNotes
		if reason != "" {
			notes = fmt.Sprintf("%s (Failed: %s)", notes, reason)
		}
		return tx.Model(&domain.Transaction{}).Where("transaction_id = ?", txID).
			Updates(map[string]interface{}{
				"status":            "failed",
				"transaction_notes": notes,
			}).Error
	})
}

