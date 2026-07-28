package usecase

import (
	"encoding/base64"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/bntngridp/ledger-backend/internal/domain"
	"github.com/bntngridp/ledger-backend/pkg/email"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"golang.org/x/crypto/bcrypt"
)

type emailOTPEntry struct {
	code      string
	expiresAt time.Time
}

type AuthUsecase interface {
	Register(username, email, password string) (*domain.RegisterResponse, error)
	Login(email, password, jwtSecret string, expiryHours int) (*domain.LoginResponse, error)
	LoginWithGoogle(profile *domain.GoogleUserProfile, jwtSecret string, expiryHours int) (*domain.LoginResponse, error)
	Generate2FASecret(userID uuid.UUID) (*domain.Enable2FAResponse, error)
	Enable2FA(userID uuid.UUID, code string) ([]string, error)
	Disable2FA(userID uuid.UUID, req domain.Disable2FARequest) error
	Send2FAEmailOTP(userID uuid.UUID) error
	SendChangePasswordEmailOTP(userID uuid.UUID) error
	ChangePassword(userID uuid.UUID, req domain.ChangePasswordRequest) error
	Verify2FALogin(preAuthToken, code, jwtSecret string, expiryHours int) (*domain.LoginResponse, error)
	Verify2FACode(userID uuid.UUID, code string) error
}

type authUsecase struct {
	userRepo      domain.UserRepository
	walletRepo    domain.WalletRepository
	emailService  email.EmailService
	encryptionKey []byte
	emailOTPs     map[uuid.UUID]emailOTPEntry
	otpMu         sync.Mutex
}

func NewAuthUsecase(userRepo domain.UserRepository, walletRepo domain.WalletRepository, emailService email.EmailService, encryptionKeyBase64 string) AuthUsecase {
	key, _ := base64.StdEncoding.DecodeString(encryptionKeyBase64)
	return &authUsecase{
		userRepo:      userRepo,
		walletRepo:    walletRepo,
		emailService:  emailService,
		encryptionKey: key,
		emailOTPs:     make(map[uuid.UUID]emailOTPEntry),
	}
}

func (uc *authUsecase) Register(username, email, password string) (*domain.RegisterResponse, error) {
	emailExists, err := uc.userRepo.CheckEmailExists(email)
	if err != nil {
		return nil, fmt.Errorf("failed to check email: %w", err)
	}
	if emailExists {
		return nil, errors.New("email already registered")
	}

	usernameExists, err := uc.userRepo.CheckUsernameExists(username)
	if err != nil {
		return nil, fmt.Errorf("failed to check username: %w", err)
	}
	if usernameExists {
		return nil, errors.New("username already taken")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	userID := uuid.New()
	walletID := uuid.New()

	hashedPasswordStr := string(hashedPassword)

	user := &domain.User{
		UserID:   userID,
		Username: username,
		Email:    email,
		Password: &hashedPasswordStr,
		IsActive: true,
	}

	wallet := &domain.Wallet{
		WalletID: walletID,
		UserID:   userID,
	}

	if err := uc.userRepo.CreateUserWithWallet(user, wallet); err != nil {
		return nil, fmt.Errorf("failed to create user with wallet: %w", err)
	}

	return &domain.RegisterResponse{
		UserID:   user.UserID.String(),
		Username: user.Username,
		Email:    user.Email,
		WalletID: walletID.String(),
		Balances: []domain.WalletBalanceDTO{
			{AssetSymbol: "IDR", Balance: decimal.Zero},
			{AssetSymbol: "USDT", Balance: decimal.Zero},
			{AssetSymbol: "USDC", Balance: decimal.Zero},
		},
	}, nil
}

type TransferUsecase interface {
	Transfer(senderUserID uuid.UUID, destUserID uuid.UUID, amount decimal.Decimal, assetSymbol string, notes string) error
}

type transferUsecase struct {
	walletRepo domain.WalletRepository
	txRepo     domain.TransactionRepository
}

func NewTransferUsecase(walletRepo domain.WalletRepository, txRepo domain.TransactionRepository) TransferUsecase {
	return &transferUsecase{
		walletRepo: walletRepo,
		txRepo:     txRepo,
	}
}

func (uc *transferUsecase) Transfer(senderUserID uuid.UUID, destUserID uuid.UUID, amount decimal.Decimal, assetSymbol string, notes string) error {
	if amount.LessThanOrEqual(decimal.Zero) {
		return errors.New("amount must be greater than 0")
	}

	if senderUserID == destUserID {
		return errors.New("cannot transfer to yourself")
	}

	senderWallet, err := uc.walletRepo.GetWalletByUserID(senderUserID)
	if err != nil {
		return fmt.Errorf("failed to get sender wallet: %w", err)
	}
	if senderWallet == nil {
		return errors.New("sender wallet not found")
	}

	recipientWallet, err := uc.walletRepo.GetWalletByUserID(destUserID)
	if err != nil {
		return fmt.Errorf("failed to get recipient wallet: %w", err)
	}
	if recipientWallet == nil {
		return errors.New("recipient wallet not found")
	}

	return uc.txRepo.ExecuteTransferTx(senderWallet.WalletID, recipientWallet.WalletID, amount, assetSymbol, notes)
}
