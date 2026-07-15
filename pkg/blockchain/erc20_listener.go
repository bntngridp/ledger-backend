// Package blockchain provides the On-Chain ERC-20 listener — a background goroutine that
// monitors ERC-20 Transfer events on a given network and credits user balances automatically.
package blockchain

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/big"
	"strings"
	"sync"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/shopspring/decimal"

	"github.com/bntngridp/ledger-backend/internal/domain"
)

const (
	minConfirmations = 3  // Minimum block confirmations before crediting a deposit
	maxBackoffSec    = 60 // Maximum reconnect delay (seconds)
)

// ListenerDeps groups all dependencies required by the ERC20Listener.
type ListenerDeps struct {
	AlchemyClient      *AlchemyClient
	CryptoAddressRepo  domain.CryptoAddressRepository
	TransactionRepo    domain.TransactionRepository
	// contractAddress -> assetSymbol, e.g., "0x..." -> "USDT"
	ContractAssets     map[string]string
	// contractAddress -> tokenDecimals, e.g., "0x..." -> 6
	ContractDecimals   map[string]int
	Network            string
}

// ERC20Listener monitors ERC-20 Transfer events and credits user deposits.
type ERC20Listener struct {
	deps        ListenerDeps
	watchList   map[string]*domain.CryptoAddress // address -> CryptoAddress
	watchMu     sync.RWMutex
}

// NewERC20Listener creates a new listener instance and pre-loads the address watch list.
func NewERC20Listener(deps ListenerDeps) *ERC20Listener {
	return &ERC20Listener{
		deps:      deps,
		watchList: make(map[string]*domain.CryptoAddress),
	}
}

// Start launches the listener in a blocking loop with exponential backoff on connection failures.
// It should be called in a goroutine: go listener.Start(ctx)
func (l *ERC20Listener) Start(ctx context.Context) {
	log.Printf("[ERC20Listener] Starting on network=%s", l.deps.Network)
	backoff := 1

	for {
		select {
		case <-ctx.Done():
			log.Println("[ERC20Listener] Context cancelled, shutting down")
			return
		default:
		}

		if err := l.refreshWatchList(ctx); err != nil {
			log.Printf("[ERC20Listener] Failed to refresh watch list: %v", err)
		}

		if err := l.listen(ctx); err != nil {
			log.Printf("[ERC20Listener] Listener error: %v. Reconnecting in %ds...", err, backoff)
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Duration(backoff) * time.Second):
			}
			// Exponential backoff capped at maxBackoffSec
			backoff = int(math.Min(float64(backoff*2), float64(maxBackoffSec)))
		} else {
			backoff = 1
		}
	}
}

// refreshWatchList loads all known deposit addresses from the database.
func (l *ERC20Listener) refreshWatchList(ctx context.Context) error {
	addresses, err := l.deps.CryptoAddressRepo.GetAllAddresses(l.deps.Network)
	if err != nil {
		return fmt.Errorf("failed to load watch list: %w", err)
	}

	l.watchMu.Lock()
	defer l.watchMu.Unlock()
	for i := range addresses {
		addr := &addresses[i]
		l.watchList[strings.ToLower(addr.Address)] = addr
	}
	log.Printf("[ERC20Listener] Watch list loaded: %d addresses", len(l.watchList))
	return nil
}

// AddToWatchList adds a new deposit address to the in-memory watch list.
// This is called when a user generates a new deposit address so the listener
// picks it up without needing a full restart.
func (l *ERC20Listener) AddToWatchList(addr *domain.CryptoAddress) {
	l.watchMu.Lock()
	defer l.watchMu.Unlock()
	l.watchList[strings.ToLower(addr.Address)] = addr
}

// listen connects to the WebSocket and processes incoming Transfer events.
func (l *ERC20Listener) listen(ctx context.Context) error {
	contractAddresses := make([]common.Address, 0, len(l.deps.ContractAssets))
	for addrStr := range l.deps.ContractAssets {
		contractAddresses = append(contractAddresses, common.HexToAddress(addrStr))
	}

	query := ethereum.FilterQuery{
		Addresses: contractAddresses,
	}

	wsClient, sub, logsChan, err := l.deps.AlchemyClient.SubscribeToLogs(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to subscribe to logs: %w", err)
	}
	defer wsClient.Close()

	log.Printf("[ERC20Listener] Subscribed to %d contract(s). Listening...", len(contractAddresses))

	for {
		select {
		case <-ctx.Done():
			return nil

		case err := <-sub.Err():
			return fmt.Errorf("subscription error: %w", err)

		case vLog := <-logsChan:
			l.handleLog(ctx, vLog)
		}
	}
}

// handleLog decodes a raw log and processes it as a deposit if applicable.
func (l *ERC20Listener) handleLog(ctx context.Context, vLog types.Log) {
	// Skip removed logs (chain reorganization)
	if vLog.Removed {
		return
	}

	event, err := DecodeTransferEvent(vLog.Data, vLog.Topics)
	if err != nil || event == nil {
		return
	}

	// Check if 'to' address belongs to one of our users.
	toAddrLower := strings.ToLower(event.To.Hex())
	l.watchMu.RLock()
	cryptoAddr, found := l.watchList[toAddrLower]
	l.watchMu.RUnlock()
	if !found {
		return
	}

	// Determine asset symbol from the contract address.
	contractLower := strings.ToLower(vLog.Address.Hex())
	assetSymbol, ok := l.deps.ContractAssets[contractLower]
	if !ok {
		return
	}

	decimals, ok := l.deps.ContractDecimals[contractLower]
	if !ok {
		decimals = 18
	}

	txHash := vLog.TxHash.Hex()

	// Wait for minimum confirmations before crediting.
	if err := l.waitForConfirmations(ctx, vLog.BlockNumber); err != nil {
		log.Printf("[ERC20Listener] Confirmation check failed for tx=%s: %v", txHash, err)
		return
	}

	// Convert token value from base units (e.g., 6 decimals for USDT) to decimal.
	divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	amountDecimal := decimal.NewFromBigInt(event.Value, 0).Div(decimal.NewFromBigInt(divisor, 0))

	notes := fmt.Sprintf("On-chain deposit on %s from %s", l.deps.Network, event.From.Hex())
	_, depositErr := l.deps.TransactionRepo.CreditCryptoDeposit(
		cryptoAddr.WalletID,
		amountDecimal,
		assetSymbol,
		txHash,
		notes,
	)

	if depositErr != nil {
		if depositErr == domain.ErrDuplicateTransaction {
			log.Printf("[ERC20Listener] Duplicate tx_hash=%s, skipping", txHash)
		} else {
			log.Printf("[ERC20Listener] Failed to credit deposit tx=%s: %v", txHash, depositErr)
		}
		return
	}

	log.Printf("[ERC20Listener] ✅ Credited %s %s to wallet=%s (tx=%s)",
		amountDecimal.String(), assetSymbol, cryptoAddr.WalletID, txHash)
}

// waitForConfirmations polls until the current block is at least minConfirmations ahead
// of the block containing the transaction.
func (l *ERC20Listener) waitForConfirmations(ctx context.Context, txBlock uint64) error {
	target := txBlock + minConfirmations
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			current, err := l.deps.AlchemyClient.GetCurrentBlock(ctx)
			if err != nil {
				log.Printf("[ERC20Listener] Block number check failed: %v", err)
				continue
			}
			if current >= target {
				return nil
			}
			log.Printf("[ERC20Listener] Waiting for confirmations: current=%d, target=%d", current, target)
		}
	}
}
