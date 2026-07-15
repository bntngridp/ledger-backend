package usecase

import (
	"github.com/bntngridp/ledger-backend/internal/domain"
	"github.com/google/uuid"
	"github.com/midtrans/midtrans-go/snap"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/mock"
)

type MockWalletRepository struct {
	mock.Mock
}

func (m *MockWalletRepository) GetWalletByUserID(userID uuid.UUID) (*domain.Wallet, error) {
	args := m.Called(userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Wallet), args.Error(1)
}

func (m *MockWalletRepository) GetWalletBalance(walletID uuid.UUID, assetSymbol string) (*domain.WalletBalance, error) {
	args := m.Called(walletID, assetSymbol)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.WalletBalance), args.Error(1)
}

func (m *MockWalletRepository) GetBalancesByWalletID(walletID uuid.UUID) ([]domain.WalletBalance, error) {
	args := m.Called(walletID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.WalletBalance), args.Error(1)
}

func (m *MockWalletRepository) GetOrCreateBalance(walletID uuid.UUID, assetSymbol string) (*domain.WalletBalance, error) {
	args := m.Called(walletID, assetSymbol)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.WalletBalance), args.Error(1)
}

type MockTransactionRepository struct {
	mock.Mock
}

func (m *MockTransactionRepository) ExecuteTransferTx(senderWalletID, recipientWalletID uuid.UUID, amount decimal.Decimal, assetSymbol, notes string) error {
	args := m.Called(senderWalletID, recipientWalletID, amount, assetSymbol, notes)
	return args.Error(0)
}

func (m *MockTransactionRepository) ExecuteTopUpTx(walletID uuid.UUID, amount decimal.Decimal, assetSymbol, notes string) (*domain.Transaction, decimal.Decimal, error) {
	args := m.Called(walletID, amount, assetSymbol, notes)
	if args.Get(0) == nil {
		return nil, args.Get(1).(decimal.Decimal), args.Error(2)
	}
	return args.Get(0).(*domain.Transaction), args.Get(1).(decimal.Decimal), args.Error(2)
}

func (m *MockTransactionRepository) GetTransactionsByWalletID(walletID uuid.UUID, page, perPage int, assetFilter, typeFilter string) ([]domain.Transaction, int64, error) {
	args := m.Called(walletID, page, perPage, assetFilter, typeFilter)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]domain.Transaction), args.Get(1).(int64), args.Error(2)
}

func (m *MockTransactionRepository) GetTransactionByOrderID(orderID string) (*domain.Transaction, error) {
	args := m.Called(orderID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Transaction), args.Error(1)
}

func (m *MockTransactionRepository) UpdateTransactionStatus(txID uuid.UUID, status string, notes string) error {
	args := m.Called(txID, status, notes)
	return args.Error(0)
}

func (m *MockTransactionRepository) CreatePendingTopUpTx(walletID uuid.UUID, amount decimal.Decimal, assetSymbol, orderID, notes string) (*domain.Transaction, error) {
	args := m.Called(walletID, amount, assetSymbol, orderID, notes)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Transaction), args.Error(1)
}

func (m *MockTransactionRepository) SettleTopUpTx(transactionID, walletID uuid.UUID, amount decimal.Decimal) error {
	args := m.Called(transactionID, walletID, amount)
	return args.Error(0)
}

func (m *MockTransactionRepository) ExecuteWithdrawFiatTx(walletID uuid.UUID, amount, adminFee decimal.Decimal, assetSymbol, notes string) (*domain.Transaction, error) {
	args := m.Called(walletID, amount, adminFee, assetSymbol, notes)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Transaction), args.Error(1)
}

func (m *MockTransactionRepository) CreditCryptoDeposit(walletID uuid.UUID, amount decimal.Decimal, assetSymbol, txHash, notes string) (*domain.Transaction, error) {
	args := m.Called(walletID, amount, assetSymbol, txHash, notes)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Transaction), args.Error(1)
}

func (m *MockTransactionRepository) CreatePendingCryptoWithdrawTx(walletID uuid.UUID, amount decimal.Decimal, assetSymbol, toAddress, notes string) (*domain.Transaction, error) {
	args := m.Called(walletID, amount, assetSymbol, toAddress, notes)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Transaction), args.Error(1)
}

func (m *MockTransactionRepository) UpdateCryptoWithdrawTx(txID uuid.UUID, txHash, status string) error {
	args := m.Called(txID, txHash, status)
	return args.Error(0)
}

func (m *MockTransactionRepository) ExecuteSwapTx(walletID uuid.UUID, fromAsset, toAsset string, fromAmount, toAmount, rateUsed, feeCharged decimal.Decimal) (*domain.Transaction, error) {
	args := m.Called(walletID, fromAsset, toAsset, fromAmount, toAmount, rateUsed, feeCharged)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Transaction), args.Error(1)
}

type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) GetUserByEmail(email string) (*domain.User, error) {
	args := m.Called(email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepository) GetUserByID(id uuid.UUID) (*domain.User, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepository) CheckEmailExists(email string) (bool, error) {
	args := m.Called(email)
	return args.Bool(0), args.Error(1)
}

func (m *MockUserRepository) CheckUsernameExists(username string) (bool, error) {
	args := m.Called(username)
	return args.Bool(0), args.Error(1)
}

func (m *MockUserRepository) CreateUserWithWallet(user *domain.User, wallet *domain.Wallet) error {
	args := m.Called(user, wallet)
	return args.Error(0)
}

func (m *MockUserRepository) GetUserByGoogleID(googleID string) (*domain.User, error) {
	args := m.Called(googleID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepository) UpdateUser(user *domain.User) error {
	args := m.Called(user)
	return args.Error(0)
}

type MockCryptoAddressRepository struct {
	mock.Mock
}

func (m *MockCryptoAddressRepository) GetAddressByWalletID(walletID uuid.UUID, network, assetSymbol string) (*domain.CryptoAddress, error) {
	args := m.Called(walletID, network, assetSymbol)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.CryptoAddress), args.Error(1)
}

func (m *MockCryptoAddressRepository) GetAddressByValue(address string) (*domain.CryptoAddress, error) {
	args := m.Called(address)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.CryptoAddress), args.Error(1)
}

func (m *MockCryptoAddressRepository) CreateAddress(cryptoAddr *domain.CryptoAddress) error {
	args := m.Called(cryptoAddr)
	return args.Error(0)
}

func (m *MockCryptoAddressRepository) GetAllAddresses(network string) ([]domain.CryptoAddress, error) {
	args := m.Called(network)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.CryptoAddress), args.Error(1)
}

type MockMidtransClient struct {
	mock.Mock
}

func (m *MockMidtransClient) CreateSnapTransaction(orderID string, amount decimal.Decimal, email string, name string) (*snap.Response, error) {
	args := m.Called(orderID, amount, email, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*snap.Response), args.Error(1)
}

func (m *MockMidtransClient) VerifySignature(orderID, statusCode, grossAmount, receivedSignature string) bool {
	args := m.Called(orderID, statusCode, grossAmount, receivedSignature)
	return args.Bool(0)
}
