// Package crypto provides utilities for generating EVM (Ethereum-compatible) wallet key pairs.
// Keys are generated from a cryptographically secure random source via go-ethereum.
package crypto

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

// KeyPair holds the generated public address and private key for an EVM wallet.
type KeyPair struct {
	// Address is the public EVM address (e.g., "0xabc...123"). Safe to share publicly.
	Address string
	// PrivateKeyHex is the hex-encoded private key. MUST be encrypted before storage.
	PrivateKeyHex string
}

// GenerateEVMKeyPair creates a new random EVM key pair using go-ethereum.
// The caller is responsible for:
//  1. Encrypting PrivateKeyHex before storing it anywhere.
//  2. Zeroing out the PrivateKeyHex string from memory after use.
func GenerateEVMKeyPair() (*KeyPair, error) {
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate EVM key pair: %w", err)
	}

	privateKeyBytes := crypto.FromECDSA(privateKey)
	address := crypto.PubkeyToAddress(privateKey.PublicKey).Hex()

	return &KeyPair{
		Address:       address,
		PrivateKeyHex: hexutil.Encode(privateKeyBytes)[2:], // strip "0x" prefix
	}, nil
}
