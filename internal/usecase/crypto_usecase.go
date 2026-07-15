package usecase

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	pkgcrypto "github.com/bntngridp/ledger-backend/pkg/crypto"
	"github.com/bntngridp/ledger-backend/internal/domain"
	"github.com/bntngridp/ledger-backend/pkg/blockchain"
	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// supportedNetworks is the whitelist of allowed blockchain networks.
var supportedNetworks = map[string]bool{
	"polygon_amoy": true,
	"sepolia":      true,
}

// supportedCryptoAssets is the whitelist of allowed crypto asset symbols.
var supportedCryptoAssets = map[string]bool{
	"USDT": true,
	"USDC": true,
}

// CryptoUsecase defines the business operations for the crypto module.
type CryptoUsecase interface {
	GetOrCreateDepositAddress(userID uuid.UUID, network, assetSymbol string) (*domain.DepositAddressResponse, error)
	WithdrawCrypto(userID uuid.UUID, req domain.CryptoWithdrawRequest) (*domain.CryptoWithdrawResponse, error)
}

type cryptoUsecase struct {
	walletRepo      domain.WalletRepository
	txRepo          domain.TransactionRepository
	cryptoAddrRepo  domain.CryptoAddressRepository
	encryptionKey   []byte
	alchemyClient   *blockchain.AlchemyClient
	contractAddrs   map[string]string  // network+asset -> contract address
	listener        *blockchain.ERC20Listener
}

// CryptoUsecaseConfig groups all dependencies for the crypto usecase.
type CryptoUsecaseConfig struct {
	WalletRepo     domain.WalletRepository
	TxRepo         domain.TransactionRepository
	CryptoAddrRepo domain.CryptoAddressRepository
	// EncryptionKeyBase64 is the base64-encoded 32-byte AES-256-GCM key from env.
	EncryptionKeyBase64 string
	AlchemyClient       *blockchain.AlchemyClient
	// ContractAddrs maps "network_asset" -> contract_address, e.g., "polygon_amoy_USDT" -> "0x..."
	ContractAddrs       map[string]string
	Listener            *blockchain.ERC20Listener
}

// NewCryptoUsecase creates a new CryptoUsecase.
func NewCryptoUsecase(cfg CryptoUsecaseConfig) (CryptoUsecase, error) {
	key, err := base64.StdEncoding.DecodeString(cfg.EncryptionKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("invalid CRYPTO_ENCRYPTION_KEY (must be base64): %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("CRYPTO_ENCRYPTION_KEY must decode to exactly 32 bytes, got %d", len(key))
	}

	return &cryptoUsecase{
		walletRepo:     cfg.WalletRepo,
		txRepo:         cfg.TxRepo,
		cryptoAddrRepo: cfg.CryptoAddrRepo,
		encryptionKey:  key,
		alchemyClient:  cfg.AlchemyClient,
		contractAddrs:  cfg.ContractAddrs,
		listener:       cfg.Listener,
	}, nil
}

// GetOrCreateDepositAddress returns an existing deposit address or generates a new one.
func (uc *cryptoUsecase) GetOrCreateDepositAddress(userID uuid.UUID, network, assetSymbol string) (*domain.DepositAddressResponse, error) {
	network = strings.ToLower(network)
	assetSymbol = strings.ToUpper(assetSymbol)

	if !supportedNetworks[network] {
		return nil, domain.ErrUnsupportedNetwork
	}
	if !supportedCryptoAssets[assetSymbol] {
		return nil, domain.ErrUnsupportedAsset
	}

	wallet, err := uc.walletRepo.GetWalletByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get wallet: %w", err)
	}
	if wallet == nil {
		return nil, domain.ErrNotFound
	}

	// Check if an address already exists for this wallet/network/asset combination.
	existing, err := uc.cryptoAddrRepo.GetAddressByWalletID(wallet.WalletID, network, assetSymbol)
	if err != nil {
		return nil, fmt.Errorf("failed to look up deposit address: %w", err)
	}
	if existing != nil {
		return &domain.DepositAddressResponse{
			Address:     existing.Address,
			Network:     existing.Network,
			AssetSymbol: existing.AssetSymbol,
		}, nil
	}

	// Generate a fresh EVM key pair.
	keyPair, err := pkgcrypto.GenerateEVMKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate EVM key pair: %w", err)
	}

	// Encrypt the private key before persisting.
	privKeyBytes, err := hex.DecodeString(keyPair.PrivateKeyHex)
	if err != nil {
		pkgcrypto.ZeroBytes([]byte(keyPair.PrivateKeyHex))
		return nil, fmt.Errorf("failed to decode private key hex: %w", err)
	}
	defer pkgcrypto.ZeroBytes(privKeyBytes)

	encryptedKey, err := pkgcrypto.Encrypt(privKeyBytes, uc.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt private key: %w", err)
	}
	// Encode ciphertext as hex for database storage.
	encryptedKeyHex := hex.EncodeToString(encryptedKey)

	cryptoAddr := &domain.CryptoAddress{
		WalletID:      wallet.WalletID,
		Network:       network,
		AssetSymbol:   assetSymbol,
		Address:       keyPair.Address,
		EncPrivateKey: encryptedKeyHex,
	}

	if err := uc.cryptoAddrRepo.CreateAddress(cryptoAddr); err != nil {
		return nil, fmt.Errorf("failed to persist deposit address: %w", err)
	}

	// Register the new address in the on-chain listener watch list immediately.
	if uc.listener != nil {
		uc.listener.AddToWatchList(cryptoAddr)
	}

	return &domain.DepositAddressResponse{
		Address:     cryptoAddr.Address,
		Network:     cryptoAddr.Network,
		AssetSymbol: cryptoAddr.AssetSymbol,
	}, nil
}

// WithdrawCrypto debits the user's balance and broadcasts an ERC-20 transfer on-chain.
func (uc *cryptoUsecase) WithdrawCrypto(userID uuid.UUID, req domain.CryptoWithdrawRequest) (*domain.CryptoWithdrawResponse, error) {
	network := strings.ToLower(req.Network)
	assetSymbol := strings.ToUpper(req.AssetSymbol)

	if !supportedNetworks[network] {
		return nil, domain.ErrUnsupportedNetwork
	}
	if !supportedCryptoAssets[assetSymbol] {
		return nil, domain.ErrUnsupportedAsset
	}

	// Validate to_address format: must be 42 chars, start with "0x".
	toAddr := strings.TrimSpace(req.ToAddress)
	if len(toAddr) != 42 || !strings.HasPrefix(strings.ToLower(toAddr), "0x") {
		return nil, domain.ErrInvalidAddress
	}
	if req.Amount.LessThanOrEqual(decimal.Zero) {
		return nil, domain.ErrInvalidInput
	}

	wallet, err := uc.walletRepo.GetWalletByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get wallet: %w", err)
	}
	if wallet == nil {
		return nil, domain.ErrNotFound
	}

	// Create pending transaction (also debits the balance atomically).
	notes := fmt.Sprintf("Crypto withdrawal to %s on %s", toAddr, network)
	txRecord, err := uc.txRepo.CreatePendingCryptoWithdrawTx(wallet.WalletID, req.Amount, assetSymbol, toAddr, notes)
	if err != nil {
		return nil, err
	}

	// If Alchemy client is configured, attempt to broadcast the transaction.
	if uc.alchemyClient != nil {
		go uc.broadcastWithdrawal(txRecord.TransactionID, wallet.WalletID, network, assetSymbol, toAddr, req.Amount)
	}

	return &domain.CryptoWithdrawResponse{
		TransactionID: txRecord.TransactionID.String(),
		AssetSymbol:   assetSymbol,
		Amount:        req.Amount,
		ToAddress:     toAddr,
		Status:        "pending",
	}, nil
}

// broadcastWithdrawal runs in a goroutine: it fetches the deposit address's private key,
// signs an ERC-20 transfer, broadcasts it, then updates the transaction record.
func (uc *cryptoUsecase) broadcastWithdrawal(txID, walletID uuid.UUID, network, assetSymbol, toAddr string, amount decimal.Decimal) {
	// Fetch the deposit address for this wallet (used as the signing key).
	cryptoAddr, err := uc.cryptoAddrRepo.GetAddressByWalletID(walletID, network, assetSymbol)
	if err != nil || cryptoAddr == nil {
		reason := "failed to fetch deposit address"
		if err != nil {
			reason = err.Error()
		}
		_ = uc.txRepo.RejectWithdrawCryptoTx(txID, reason)
		return
	}

	// Determine contract address.
	contractKey := network + "_" + assetSymbol
	contractAddrStr, ok := uc.contractAddrs[contractKey]
	if !ok {
		_ = uc.txRepo.RejectWithdrawCryptoTx(txID, "contract address not configured for "+contractKey)
		return
	}

	// Decrypt private key.
	encBytes, err := hex.DecodeString(cryptoAddr.EncPrivateKey)
	if err != nil {
		_ = uc.txRepo.RejectWithdrawCryptoTx(txID, "failed to decode encrypted private key: "+err.Error())
		return
	}
	privKeyBytes, err := pkgcrypto.Decrypt(encBytes, uc.encryptionKey)
	if err != nil {
		_ = uc.txRepo.RejectWithdrawCryptoTx(txID, "failed to decrypt private key: "+err.Error())
		return
	}
	defer pkgcrypto.ZeroBytes(privKeyBytes)

	privKey, err := blockchain.PrivateKeyFromHex(hex.EncodeToString(privKeyBytes))
	if err != nil {
		_ = uc.txRepo.RejectWithdrawCryptoTx(txID, "failed to parse private key: "+err.Error())
		return
	}

	import_ctx := context.Background()
	chainID, err := uc.alchemyClient.GetChainID(import_ctx)
	if err != nil {
		_ = uc.txRepo.RejectWithdrawCryptoTx(txID, "failed to get chain ID: "+err.Error())
		return
	}

	fromAddr := common.HexToAddress(cryptoAddr.Address)
	nonce, err := uc.alchemyClient.GetTransactionCount(import_ctx, fromAddr)
	if err != nil {
		_ = uc.txRepo.RejectWithdrawCryptoTx(txID, "failed to get nonce: "+err.Error())
		return
	}

	gasPrice, err := uc.alchemyClient.SuggestGasPrice(import_ctx)
	if err != nil {
		_ = uc.txRepo.RejectWithdrawCryptoTx(txID, "failed to suggest gas price: "+err.Error())
		return
	}

	// Convert decimal amount to token base units (USDT/USDC = 6 decimals on most testnets).
	// Using 6 decimals as the standard for stablecoins.
	tokenDecimals := int64(6)
	divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(tokenDecimals), nil)
	amountBig := amount.Mul(decimal.NewFromBigInt(divisor, 0)).BigInt()

	params := blockchain.ERC20TransferTxParams{
		PrivateKey:      privKey,
		ContractAddress: common.HexToAddress(contractAddrStr),
		ToAddress:       common.HexToAddress(toAddr),
		Amount:          amountBig,
		Nonce:           nonce,
		GasPrice:        gasPrice,
		ChainID:         chainID,
	}

	signedTx, err := blockchain.BuildSignedERC20Transfer(params)
	if err != nil {
		_ = uc.txRepo.RejectWithdrawCryptoTx(txID, "failed to build transfer tx: "+err.Error())
		return
	}

	if err := uc.alchemyClient.SendSignedTransaction(import_ctx, signedTx); err != nil {
		_ = uc.txRepo.RejectWithdrawCryptoTx(txID, "failed to broadcast tx: "+err.Error())
		return
	}

	txHash := signedTx.Hash().Hex()
	_ = uc.txRepo.UpdateCryptoWithdrawTx(txID, txHash, "success")
}
