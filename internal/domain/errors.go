package domain

import "errors"

var (
	// ErrNotFound is returned when a requested resource does not exist.
	ErrNotFound = errors.New("resource not found")

	// ErrUnauthorized is returned when authentication credentials are missing or invalid.
	ErrUnauthorized = errors.New("unauthorized access")

	// ErrForbidden is returned when the authenticated user lacks permission.
	ErrForbidden = errors.New("forbidden")

	// ErrConflict is returned when a resource already exists (e.g., duplicate email).
	ErrConflict = errors.New("resource already exists")

	// ErrInsufficientBalance is returned when a user doesn't have enough funds.
	ErrInsufficientBalance = errors.New("insufficient balance")

	// ErrInvalidInput is returned when the provided input fails validation.
	ErrInvalidInput = errors.New("invalid input")

	// ErrDuplicateTransaction is returned when an idempotent operation is attempted twice.
	// The caller should treat this as success (200 OK), not an error.
	ErrDuplicateTransaction = errors.New("transaction already processed")

	// ErrExternalService is returned when a downstream service (Midtrans, Alchemy) fails.
	ErrExternalService = errors.New("external service error")

	// ErrInvalidSignature is returned when a webhook signature validation fails.
	ErrInvalidSignature = errors.New("invalid webhook signature")

	// ErrSelfTransfer is returned when a user tries to transfer to themselves.
	ErrSelfTransfer = errors.New("cannot transfer to yourself")

	// ErrInvalidAddress is returned when an EVM wallet address is malformed.
	ErrInvalidAddress = errors.New("invalid wallet address")

	// ErrUnsupportedAsset is returned when the requested asset symbol is not supported.
	ErrUnsupportedAsset = errors.New("unsupported asset")

	// ErrUnsupportedNetwork is returned when the requested blockchain network is not supported.
	ErrUnsupportedNetwork = errors.New("unsupported network")

	// ErrRateUnavailable is returned when exchange rate data cannot be fetched.
	ErrRateUnavailable = errors.New("exchange rate unavailable")

	// ErrSameAssetSwap is returned when from_asset equals to_asset in a swap.
	ErrSameAssetSwap = errors.New("cannot swap asset to itself")
)
