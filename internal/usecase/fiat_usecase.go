package usecase

import (
	"fmt"
	"strings"

	"github.com/bntngridp/ledger-backend/internal/domain"
	"github.com/bntngridp/ledger-backend/pkg/midtrans"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

const (
	minFiatWithdrawAmount = 50000 // Minimum Rp 50.000
	fiatWithdrawAdminFee  = 2500  // Flat Rp 2.500 admin fee
)

// FiatUsecase defines the business operations for fiat withdrawals and transfers.
type FiatUsecase interface {
	WithdrawFiat(userID uuid.UUID, req domain.WithdrawFiatRequest) (*domain.WithdrawFiatResponse, error)
}

type fiatUsecase struct {
	walletRepo domain.WalletRepository
	txRepo     domain.TransactionRepository
	irisClient *midtrans.IrisClient
}

// NewFiatUsecase constructs a FiatUsecase.
func NewFiatUsecase(
	walletRepo domain.WalletRepository,
	txRepo domain.TransactionRepository,
	irisClient *midtrans.IrisClient,
) FiatUsecase {
	return &fiatUsecase{
		walletRepo: walletRepo,
		txRepo:     txRepo,
		irisClient: irisClient,
	}
}

func (uc *fiatUsecase) WithdrawFiat(userID uuid.UUID, req domain.WithdrawFiatRequest) (*domain.WithdrawFiatResponse, error) {
	if req.Amount.LessThan(decimal.NewFromInt(minFiatWithdrawAmount)) {
		return nil, fmt.Errorf("%w: minimum withdrawal amount is Rp %d", domain.ErrInvalidInput, minFiatWithdrawAmount)
	}

	wallet, err := uc.walletRepo.GetWalletByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get wallet: %w", err)
	}
	if wallet == nil {
		return nil, domain.ErrNotFound
	}

	// Verify IDR balance is sufficient (amount + admin fee)
	bal, err := uc.walletRepo.GetWalletBalance(wallet.WalletID, "IDR")
	if err != nil {
		return nil, fmt.Errorf("failed to check balance: %w", err)
	}

	adminFee := decimal.NewFromInt(fiatWithdrawAdminFee)
	totalDeducted := req.Amount.Add(adminFee)

	if bal == nil || bal.Balance.LessThan(totalDeducted) {
		return nil, domain.ErrInsufficientBalance
	}

	// Round everything to integer Rupiah
	req.Amount = req.Amount.Round(0)
	adminFee = adminFee.Round(0)
	totalDeducted = totalDeducted.Round(0)

	// Execute database transaction to debit the balance and write transaction
	notes := fmt.Sprintf("Withdrawal to %s bank account %s. Notes: %s", req.BankCode, req.AccountNumber, req.Notes)
	txRecord, err := uc.txRepo.ExecuteWithdrawFiatTx(wallet.WalletID, req.Amount, adminFee, "IDR", notes)
	if err != nil {
		return nil, err
	}

	status := "pending"

	// If Midtrans Iris client is configured, send the payout request
	if uc.irisClient != nil {
		item := midtrans.IrisPayoutItem{
			BeneficiaryName:          req.AccountName,
			BeneficiaryAccountNumber: req.AccountNumber,
			BeneficiaryBankCode:      req.BankCode,
			Amount:                   req.Amount.StringFixed(0),
			Notes:                    notes,
		}

		payoutResp, irisErr := uc.irisClient.CreatePayout(item)
		if irisErr != nil {
			// Update status to failed in database
			_ = uc.txRepo.UpdateTransactionStatus(txRecord.TransactionID, "failed", "Iris API failure: "+irisErr.Error())
			return nil, fmt.Errorf("%w: %v", domain.ErrExternalService, irisErr)
		}

		if len(payoutResp.Payouts) > 0 {
			// Update status from Iris
			status = strings.ToLower(payoutResp.Payouts[0].Status)
			if status == "success" || status == "processed" {
				status = "success"
			} else {
				status = "pending" // Let polling/callback handle final state
			}
			_ = uc.txRepo.UpdateTransactionStatus(txRecord.TransactionID, status, "")
		}
	}

	return &domain.WithdrawFiatResponse{
		TransactionID: txRecord.TransactionID.String(),
		Amount:        req.Amount,
		AdminFee:      adminFee,
		TotalDeducted: totalDeducted,
		BankCode:      req.BankCode,
		AccountNumber: req.AccountNumber,
		Status:        status,
	}, nil
}
