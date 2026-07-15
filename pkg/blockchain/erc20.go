// Package blockchain provides ERC-20 ABI utilities and transaction building helpers.
package blockchain

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

// ERC20TransferABI is the minimal ABI fragment required to parse Transfer events.
const ERC20TransferABI = `[{"anonymous":false,"inputs":[{"indexed":true,"name":"from","type":"address"},{"indexed":true,"name":"to","type":"address"},{"indexed":false,"name":"value","type":"uint256"}],"name":"Transfer","type":"event"},{"inputs":[{"name":"to","type":"address"},{"name":"amount","type":"uint256"}],"name":"transfer","outputs":[{"name":"","type":"bool"}],"stateMutability":"nonpayable","type":"function"}]`

// ParsedERC20ABI holds the parsed ABI object for decoding ERC-20 events.
var ParsedERC20ABI abi.ABI

func init() {
	var err error
	ParsedERC20ABI, err = abi.JSON(strings.NewReader(ERC20TransferABI))
	if err != nil {
		panic(fmt.Sprintf("failed to parse ERC-20 ABI: %v", err))
	}
}

// TransferEvent represents a decoded ERC-20 Transfer(from, to, value) event.
type TransferEvent struct {
	From  common.Address
	To    common.Address
	Value *big.Int
}

// DecodeTransferEvent decodes a raw log into a TransferEvent.
// Returns (nil, nil) if the log topic does not match the Transfer event signature.
func DecodeTransferEvent(logData []byte, topics []common.Hash) (*TransferEvent, error) {
	transferSig := ParsedERC20ABI.Events["Transfer"].ID
	if len(topics) < 3 || topics[0] != transferSig {
		return nil, nil // Not a Transfer event
	}

	from := common.HexToAddress(topics[1].Hex())
	to := common.HexToAddress(topics[2].Hex())

	var value struct {
		Value *big.Int
	}
	if err := ParsedERC20ABI.UnpackIntoInterface(&value, "Transfer", logData); err != nil {
		return nil, fmt.Errorf("failed to unpack Transfer event: %w", err)
	}

	return &TransferEvent{From: from, To: to, Value: value.Value}, nil
}

// ERC20TransferTxParams holds all parameters needed to build a signed ERC-20 transfer tx.
type ERC20TransferTxParams struct {
	PrivateKey      *ecdsa.PrivateKey
	ContractAddress common.Address
	ToAddress       common.Address
	Amount          *big.Int
	Nonce           uint64
	GasPrice        *big.Int
	GasLimit        uint64
	ChainID         *big.Int
}

// BuildSignedERC20Transfer constructs and signs an ERC-20 transfer() call.
// The caller is responsible for broadcasting the returned transaction.
func BuildSignedERC20Transfer(params ERC20TransferTxParams) (*types.Transaction, error) {
	// Encode the transfer(address, uint256) function call
	data, err := ParsedERC20ABI.Pack("transfer", params.ToAddress, params.Amount)
	if err != nil {
		return nil, fmt.Errorf("failed to pack ERC-20 transfer data: %w", err)
	}

	gasLimit := params.GasLimit
	if gasLimit == 0 {
		gasLimit = 100_000 // Safe default for ERC-20 transfers
	}

	tx := types.NewTransaction(
		params.Nonce,
		params.ContractAddress,
		big.NewInt(0), // No ETH value — this is a token transfer
		gasLimit,
		params.GasPrice,
		data,
	)

	signer := types.NewLondonSigner(params.ChainID)
	signedTx, err := types.SignTx(tx, signer, params.PrivateKey)
	if err != nil {
		// Zero out the sensitive key reference before returning
		zeroPrivateKey(params.PrivateKey)
		return nil, fmt.Errorf("failed to sign transaction: %w", err)
	}

	// Zero out the private key from the params struct after signing
	zeroPrivateKey(params.PrivateKey)

	return signedTx, nil
}

// PrivateKeyFromHex converts a hex private key string to an ECDSA private key.
// The caller is responsible for zeroing the raw hex string after this call.
func PrivateKeyFromHex(hexKey string) (*ecdsa.PrivateKey, error) {
	privateKey, err := crypto.HexToECDSA(hexKey)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}
	return privateKey, nil
}

// zeroPrivateKey overwrites the D scalar of the private key (best-effort memory zeroing).
func zeroPrivateKey(key *ecdsa.PrivateKey) {
	if key != nil && key.D != nil {
		key.D.SetInt64(0)
	}
}
