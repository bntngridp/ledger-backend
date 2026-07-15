package usecase

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/bntngridp/ledger-backend/internal/domain"
	"github.com/bntngridp/ledger-backend/pkg/midtrans"
	"github.com/bntngridp/ledger-backend/pkg/price"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// WalletUsecase defines the business operations for the wallet module.
type WalletUsecase interface {
	TopUp(userID uuid.UUID, amount decimal.Decimal, assetSymbol, notes string) (*domain.TopUpResponse, error)
	GetTransactionHistory(userID uuid.UUID, page, perPage int, assetFilter, typeFilter string) (*domain.TransactionHistoryResponse, error)
	GetDashboard(userID uuid.UUID) (*domain.DashboardResponse, error)
}

type walletUsecase struct {
	walletRepo     domain.WalletRepository
	txRepo         domain.TransactionRepository
	midtransClient midtrans.Client
	priceCache     *price.PriceCache
}

// NewWalletUsecase creates a new WalletUsecase.
func NewWalletUsecase(
	walletRepo domain.WalletRepository,
	txRepo domain.TransactionRepository,
	midtransClient midtrans.Client,
	priceCache *price.PriceCache,
) WalletUsecase {
	return &walletUsecase{
		walletRepo:     walletRepo,
		txRepo:         txRepo,
		midtransClient: midtransClient,
		priceCache:     priceCache,
	}
}

func (uc *walletUsecase) TopUp(userID uuid.UUID, amount decimal.Decimal, assetSymbol, notes string) (*domain.TopUpResponse, error) {
	if amount.LessThanOrEqual(decimal.Zero) {
		return nil, domain.ErrInvalidInput
	}

	wallet, err := uc.walletRepo.GetWalletByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get wallet: %w", err)
	}
	if wallet == nil {
		return nil, domain.ErrNotFound
	}

	orderID := fmt.Sprintf("TOPUP-IDR-%s-%d", wallet.WalletID.String()[:8], time.Now().UnixNano())

	txRecord, err := uc.txRepo.CreatePendingTopUpTx(wallet.WalletID, amount, assetSymbol, orderID, notes)
	if err != nil {
		return nil, fmt.Errorf("failed to record pending transaction: %w", err)
	}

	email := "user@example.com"
	name := "User"
	if wallet.User != nil {
		email = wallet.User.Email
		name = wallet.User.Username
	}

	snapResp, err := uc.midtransClient.CreateSnapTransaction(orderID, amount, email, name)
	if err != nil {
		_ = uc.txRepo.UpdateTransactionStatus(txRecord.TransactionID, "failed", "Midtrans charge failed: "+err.Error())
		return nil, fmt.Errorf("%w: %v", domain.ErrExternalService, err)
	}

	return &domain.TopUpResponse{
		TransactionID: txRecord.TransactionID.String(),
		WalletID:      wallet.WalletID.String(),
		AssetSymbol:   assetSymbol,
		Amount:        amount,
		SnapToken:     snapResp.Token,
		RedirectURL:   snapResp.RedirectURL,
	}, nil
}

func (uc *walletUsecase) GetTransactionHistory(userID uuid.UUID, page, perPage int, assetFilter, typeFilter string) (*domain.TransactionHistoryResponse, error) {
	wallet, err := uc.walletRepo.GetWalletByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get wallet: %w", err)
	}
	if wallet == nil {
		return nil, domain.ErrNotFound
	}

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	transactions, total, err := uc.txRepo.GetTransactionsByWalletID(wallet.WalletID, page, perPage, assetFilter, typeFilter)
	if err != nil {
		return nil, err
	}

	totalPages := int(math.Ceil(float64(total) / float64(perPage)))
	if totalPages == 0 {
		totalPages = 1
	}

	var items []domain.TransactionHistoryItem
	for _, t := range transactions {
		item := domain.TransactionHistoryItem{
			TransactionID:    t.TransactionID.String(),
			AssetSymbol:      t.AssetSymbol,
			Amount:           t.Amount,
			Type:             t.Type,
			Status:           t.Status,
			TransactionNotes: t.TransactionNotes,
			TxHash:           t.TxHash,
			MidtransOrderID:  t.MidtransOrderID,
			RateUsed:         t.RateUsed,
			FeeCharged:       t.FeeCharged,
			CreatedAt:        t.CreatedAt,
		}
		if t.SourceWalletID != nil {
			s := t.SourceWalletID.String()
			item.SourceWalletID = &s
		}
		if t.DestinationWalletID != nil {
			d := t.DestinationWalletID.String()
			item.DestinationWalletID = &d
		}
		items = append(items, item)
	}
	if items == nil {
		items = []domain.TransactionHistoryItem{}
	}

	return &domain.TransactionHistoryResponse{
		Transactions: items,
		Meta: domain.PaginationMeta{
			Page:       page,
			PerPage:    perPage,
			Total:      total,
			TotalPages: totalPages,
		},
	}, nil
}

func (uc *walletUsecase) GetDashboard(userID uuid.UUID) (*domain.DashboardResponse, error) {
	wallet, err := uc.walletRepo.GetWalletByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get wallet: %w", err)
	}
	if wallet == nil {
		return nil, errors.New("wallet not found")
	}

	var balancesDTO []domain.WalletBalanceDTO
	estimatedTotalIDR := decimal.Zero

	for _, b := range wallet.Balances {
		balancesDTO = append(balancesDTO, domain.WalletBalanceDTO{
			AssetSymbol: b.AssetSymbol,
			Balance:     b.Balance,
		})

		switch b.AssetSymbol {
		case "IDR":
			estimatedTotalIDR = estimatedTotalIDR.Add(b.Balance)
		case "USDT", "USDC":
			pair := b.AssetSymbol + "_IDR"
			rate, _, rateErr := uc.priceCache.GetRate(pair)
			if rateErr == nil && rate.IsPositive() {
				estimatedTotalIDR = estimatedTotalIDR.Add(b.Balance.Mul(rate))
			}
		}
	}

	if balancesDTO == nil {
		balancesDTO = []domain.WalletBalanceDTO{}
	}

	return &domain.DashboardResponse{
		WalletID:          wallet.WalletID.String(),
		Balances:          balancesDTO,
		EstimatedTotalIDR: estimatedTotalIDR,
	}, nil
}
