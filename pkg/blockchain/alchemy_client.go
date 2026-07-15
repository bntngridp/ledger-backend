// Package blockchain provides utilities for interacting with EVM-compatible blockchains
// via Alchemy RPC (HTTP and WebSocket).
package blockchain

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// AlchemyClient wraps the go-ethereum ethclient for communicating with Alchemy RPC nodes.
type AlchemyClient struct {
	httpURL string
	wsURL   string
}

// NewAlchemyClient creates a new AlchemyClient configured with the given RPC URLs.
func NewAlchemyClient(httpURL, wsURL string) *AlchemyClient {
	return &AlchemyClient{
		httpURL: httpURL,
		wsURL:   wsURL,
	}
}

// newHTTPClient dials a fresh HTTP ethclient connection.
func (c *AlchemyClient) newHTTPClient(ctx context.Context) (*ethclient.Client, error) {
	client, err := ethclient.DialContext(ctx, c.httpURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Alchemy HTTP RPC: %w", err)
	}
	return client, nil
}

// newWSClient dials a fresh WebSocket ethclient connection.
func (c *AlchemyClient) newWSClient(ctx context.Context) (*ethclient.Client, error) {
	client, err := ethclient.DialContext(ctx, c.wsURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Alchemy WebSocket RPC: %w", err)
	}
	return client, nil
}

// GetCurrentBlock returns the latest block number from the chain.
func (c *AlchemyClient) GetCurrentBlock(ctx context.Context) (uint64, error) {
	client, err := c.newHTTPClient(ctx)
	if err != nil {
		return 0, err
	}
	defer client.Close()

	blockNum, err := client.BlockNumber(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get block number: %w", err)
	}
	return blockNum, nil
}

// GetTransactionReceipt fetches the receipt for a given transaction hash.
// Returns (nil, nil) if the transaction has not been mined yet.
func (c *AlchemyClient) GetTransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error) {
	client, err := c.newHTTPClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	receipt, err := client.TransactionReceipt(ctx, txHash)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get receipt: %w", err)
	}
	return receipt, nil
}

// SendSignedTransaction broadcasts a signed transaction to the network.
func (c *AlchemyClient) SendSignedTransaction(ctx context.Context, signedTx *types.Transaction) error {
	client, err := c.newHTTPClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	if err := client.SendTransaction(ctx, signedTx); err != nil {
		return fmt.Errorf("failed to broadcast transaction: %w", err)
	}
	return nil
}

// GetTransactionCount returns the nonce for the given address (pending state).
func (c *AlchemyClient) GetTransactionCount(ctx context.Context, address common.Address) (uint64, error) {
	client, err := c.newHTTPClient(ctx)
	if err != nil {
		return 0, err
	}
	defer client.Close()

	nonce, err := client.PendingNonceAt(ctx, address)
	if err != nil {
		return 0, fmt.Errorf("failed to get nonce: %w", err)
	}
	return nonce, nil
}

// SuggestGasPrice returns the current suggested gas price from the node.
func (c *AlchemyClient) SuggestGasPrice(ctx context.Context) (*big.Int, error) {
	client, err := c.newHTTPClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	price, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to suggest gas price: %w", err)
	}
	return price, nil
}

// GetChainID returns the chain ID of the connected network.
func (c *AlchemyClient) GetChainID(ctx context.Context) (*big.Int, error) {
	client, err := c.newHTTPClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	chainID, err := client.ChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get chain ID: %w", err)
	}
	return chainID, nil
}

// SubscribeToLogs opens a WebSocket subscription for contract logs matching the given filter.
// The caller is responsible for handling reconnects via the ERC20Listener.
func (c *AlchemyClient) SubscribeToLogs(ctx context.Context, query ethereum.FilterQuery) (*ethclient.Client, ethereum.Subscription, chan types.Log, error) {
	client, err := c.newWSClient(ctx)
	if err != nil {
		return nil, nil, nil, err
	}

	logs := make(chan types.Log, 100)
	sub, err := client.SubscribeFilterLogs(ctx, query, logs)
	if err != nil {
		client.Close()
		return nil, nil, nil, fmt.Errorf("failed to subscribe to logs: %w", err)
	}
	return client, sub, logs, nil
}
