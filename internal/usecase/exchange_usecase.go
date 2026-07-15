package usecase

import (
	"fmt"
	"strings"

	"github.com/bntngridp/ledger-backend/internal/domain"
	"github.com/bntngridp/ledger-backend/pkg/price"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

var allowedAssets = map[string]bool{
	"IDR":  true,
	"USDT": true,
	"USDC": true,
}

// ExchangeUsecase defines the business operations for swap and rate feeds.
type ExchangeUsecase interface {
	GetExchangeRate(pair string) (*domain.ExchangeRateResponse, error)
	Swap(userID uuid.UUID, req domain.SwapRequest) (*domain.SwapResponse, error)
}

type exchangeUsecase struct {
	walletRepo    domain.WalletRepository
	txRepo        domain.TransactionRepository
	priceCache    *price.PriceCache
	feePercentage decimal.Decimal // e.g. 0.005 for 0.5%
}

// NewExchangeUsecase constructs an ExchangeUsecase.
func NewExchangeUsecase(
	walletRepo domain.WalletRepository,
	txRepo domain.TransactionRepository,
	priceCache *price.PriceCache,
	feePercentage decimal.Decimal,
) ExchangeUsecase {
	if feePercentage.IsZero() {
		feePercentage = decimal.NewFromFloat(0.005) // Default 0.5%
	}
	return &exchangeUsecase{
		walletRepo:    walletRepo,
		txRepo:        txRepo,
		priceCache:    priceCache,
		feePercentage: feePercentage,
	}
}

func (uc *exchangeUsecase) GetExchangeRate(pair string) (*domain.ExchangeRateResponse, error) {
	pair = strings.ToUpper(pair)
	parts := strings.Split(pair, "_")
	if len(parts) != 2 || !allowedAssets[parts[0]] || !allowedAssets[parts[1]] {
		return nil, domain.ErrInvalidInput
	}

	// Fetch exchange rate from PriceCache
	rate, fetchedAt, err := uc.priceCache.GetRate(pair)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrRateUnavailable, err)
	}

	return &domain.ExchangeRateResponse{
		Pair:        pair,
		Rate:        rate,
		LastUpdated: fetchedAt,
	}, nil
}

func (uc *exchangeUsecase) Swap(userID uuid.UUID, req domain.SwapRequest) (*domain.SwapResponse, error) {
	fromAsset := strings.ToUpper(req.FromAsset)
	toAsset := strings.ToUpper(req.ToAsset)

	if !allowedAssets[fromAsset] || !allowedAssets[toAsset] {
		return nil, domain.ErrUnsupportedAsset
	}
	if fromAsset == toAsset {
		return nil, domain.ErrSameAssetSwap
	}
	if req.Amount.LessThanOrEqual(decimal.Zero) {
		return nil, domain.ErrInvalidInput
	}

	// Fetch rates relative to IDR
	var fromRate, toRate decimal.Decimal
	if fromAsset == "IDR" {
		fromRate = decimal.NewFromInt(1)
	} else {
		rate, _, err := uc.priceCache.GetRate(fromAsset + "_IDR")
		if err != nil {
			return nil, domain.ErrRateUnavailable
		}
		fromRate = rate
	}

	if toAsset == "IDR" {
		toRate = decimal.NewFromInt(1)
	} else {
		rate, _, err := uc.priceCache.GetRate(toAsset + "_IDR")
		if err != nil {
			return nil, domain.ErrRateUnavailable
		}
		toRate = rate
	}

	wallet, err := uc.walletRepo.GetWalletByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get wallet: %w", err)
	}
	if wallet == nil {
		return nil, domain.ErrNotFound
	}

	// Verify fromAsset balance
	bal, err := uc.walletRepo.GetWalletBalance(wallet.WalletID, fromAsset)
	if err != nil {
		return nil, fmt.Errorf("failed to verify balance: %w", err)
	}
	if bal == nil || bal.Balance.LessThan(req.Amount) {
		return nil, domain.ErrInsufficientBalance
	}

	// Calculate Swap amounts
	// grossToAmount = (req.Amount * fromRate) / toRate
	grossToAmount := req.Amount.Mul(fromRate).Div(toRate)
	crossRate := fromRate.Div(toRate)

	// Platform fee (0.5%)
	feeCharged := grossToAmount.Mul(uc.feePercentage)
	netToAmount := grossToAmount.Sub(feeCharged)

	// Round IDR amounts to full Rupiah (no decimal cents)
	if toAsset == "IDR" {
		netToAmount = netToAmount.Round(0)
		feeCharged = feeCharged.Round(0)
	}
	if fromAsset == "IDR" {
		// Just ensure input amount was integer
		req.Amount = req.Amount.Round(0)
	}

	// Execute atomic swap transaction in repository (locks balance rows)
	txRecord, err := uc.txRepo.ExecuteSwapTx(wallet.WalletID, fromAsset, toAsset, req.Amount, netToAmount, crossRate, feeCharged)
	if err != nil {
		return nil, err
	}

	return &domain.SwapResponse{
		TransactionID: txRecord.TransactionID.String(),
		FromAsset:     fromAsset,
		ToAsset:       toAsset,
		FromAmount:    req.Amount,
		ToAmount:      netToAmount,
		RateUsed:      crossRate,
		FeeCharged:    feeCharged,
	}, nil
}
