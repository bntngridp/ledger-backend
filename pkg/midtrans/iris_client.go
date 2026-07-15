// Package midtrans provides a client for the Midtrans Iris Disbursement API (Sandbox).
// Iris is Midtrans's platform for sending money to bank accounts.
package midtrans

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// IrisClient wraps the Midtrans Iris Sandbox API for fiat disbursements.
type IrisClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// NewIrisClient creates a new IrisClient for the Midtrans Iris Sandbox.
//   - apiKey: Midtrans Iris Sandbox API Key
//   - baseURL: e.g., "https://app.sandbox.midtrans.com/iris"
func NewIrisClient(apiKey, baseURL string) *IrisClient {
	return &IrisClient{
		apiKey:  apiKey,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// IrisBeneficiary represents the recipient of a payout.
type IrisBeneficiary struct {
	Name          string `json:"name"`
	AccountNumber string `json:"account"`
	BankCode      string `json:"bank_account_type"` // e.g., "bca", "mandiri"
	AliasName     string `json:"alias_name"`
	Email         string `json:"email,omitempty"`
}

// IrisPayoutRequest represents a Midtrans Iris payout request body.
type IrisPayoutRequest struct {
	Payouts []IrisPayoutItem `json:"payouts"`
}

// IrisPayoutItem is a single payout entry within a batch.
type IrisPayoutItem struct {
	BeneficiaryName          string `json:"beneficiary_name"`
	BeneficiaryAccountNumber string `json:"beneficiary_account"`
	BeneficiaryBankCode      string `json:"beneficiary_bank"`
	BeneficiaryEmail         string `json:"beneficiary_email,omitempty"`
	Amount                   string `json:"amount"` // in Rupiah, as string
	Notes                    string `json:"notes,omitempty"`
}

// IrisPayoutResponse is the API response for a create payout request.
type IrisPayoutResponse struct {
	Payouts []struct {
		Status        string `json:"status"`
		ReferenceNo   string `json:"reference_no"`
		Amount        string `json:"amount"`
		BeneficiaryName string `json:"beneficiary_name"`
	} `json:"payouts"`
}

// CreatePayout submits a single disbursement request to Midtrans Iris Sandbox.
func (c *IrisClient) CreatePayout(req IrisPayoutItem) (*IrisPayoutResponse, error) {
	body, err := json.Marshal(IrisPayoutRequest{Payouts: []IrisPayoutItem{req}})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal iris request: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, c.baseURL+"/api/v1/payouts", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create iris request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	// Iris uses HTTP Basic Auth: API Key as username, no password
	encoded := base64.StdEncoding.EncodeToString([]byte(c.apiKey + ":"))
	httpReq.Header.Set("Authorization", "Basic "+encoded)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("iris API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read iris response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("iris API returned error status %d: %s", resp.StatusCode, string(respBody))
	}

	var payoutResp IrisPayoutResponse
	if err := json.Unmarshal(respBody, &payoutResp); err != nil {
		return nil, fmt.Errorf("failed to parse iris response: %w", err)
	}

	return &payoutResp, nil
}
