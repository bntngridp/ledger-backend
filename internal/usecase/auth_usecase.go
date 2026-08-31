package usecase

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
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

type biometricChallengeEntry struct {
	challenge string
	expiresAt time.Time
}

type AuthUsecase interface {
	Register(username, email, password string) (*domain.RegisterResponse, error)
	Login(email, password, jwtSecret string, expiryHours int) (*domain.LoginResponse, error)
	LoginWithGoogle(profile *domain.GoogleUserProfile, jwtSecret string, expiryHours int) (*domain.LoginResponse, error)
	Generate2FASecret(userID uuid.UUID) (*domain.Enable2FAResponse, error)
	Enable2FA(userID uuid.UUID, code string) ([]string, error)
	Disable2FA(userID uuid.UUID, req domain.Disable2FARequest) error
	GetRecoveryCodes(userID uuid.UUID) ([]string, error)
	RegenerateRecoveryCodes(userID uuid.UUID) ([]string, error)
	Send2FAEmailOTP(userID uuid.UUID) error
	SendPaymentEmailOTP(userID uuid.UUID) error
	SendChangePasswordEmailOTP(userID uuid.UUID) error
	ChangePassword(userID uuid.UUID, req domain.ChangePasswordRequest) error
	Verify2FALogin(preAuthToken, code, jwtSecret string, expiryHours int) (*domain.LoginResponse, error)
	Verify2FACode(userID uuid.UUID, code string) error
	VerifyPaymentSecurity(userID uuid.UUID, twoFactorCode string, emailOTP string) error
	SetupPIN(userID uuid.UUID, pin string) error
	VerifyPIN(userID uuid.UUID, pin string) error
	GetBiometricChallenge(userID uuid.UUID) (string, error)
	RegisterBiometric(userID uuid.UUID, req domain.BiometricRegisterRequest) error
	VerifyBiometric(userID uuid.UUID, req domain.BiometricVerifyRequest) error
	DisableBiometric(userID uuid.UUID) error
	GetMe(userID uuid.UUID) (*domain.UserProfileResponse, error)
}

type authUsecase struct {
	userRepo           domain.UserRepository
	walletRepo         domain.WalletRepository
	emailService       email.EmailService
	encryptionKey      []byte
	emailOTPs          map[uuid.UUID]emailOTPEntry
	otpMu              sync.Mutex
	biometricChallenges map[uuid.UUID]biometricChallengeEntry
	biometricMu        sync.Mutex
}

func NewAuthUsecase(userRepo domain.UserRepository, walletRepo domain.WalletRepository, emailService email.EmailService, encryptionKeyBase64 string) AuthUsecase {
	key, _ := base64.StdEncoding.DecodeString(encryptionKeyBase64)
	return &authUsecase{
		userRepo:            userRepo,
		walletRepo:          walletRepo,
		emailService:        emailService,
		encryptionKey:       key,
		emailOTPs:           make(map[uuid.UUID]emailOTPEntry),
		biometricChallenges: make(map[uuid.UUID]biometricChallengeEntry),
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

func (uc *authUsecase) SetupPIN(userID uuid.UUID, pin string) error {
	if len(pin) != 6 {
		return errors.New("PIN harus 6 digit angka")
	}

	user, err := uc.userRepo.GetUserByID(userID)
	if err != nil || user == nil {
		return errors.New("user not found")
	}

	hashedPIN, err := bcrypt.GenerateFromPassword([]byte(pin), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash PIN: %w", err)
	}

	pinStr := string(hashedPIN)
	user.TransactionPIN = &pinStr
	user.PINEnabled = true

	return uc.userRepo.UpdateUser(user)
}

func (uc *authUsecase) VerifyPIN(userID uuid.UUID, pin string) error {
	if len(pin) != 6 {
		return errors.New("PIN transaksi harus 6 digit")
	}

	user, err := uc.userRepo.GetUserByID(userID)
	if err != nil || user == nil {
		return errors.New("user tidak ditemukan")
	}

	// Default PIN check: if PIN has not been custom set, allow 123456 as standard initial PIN
	if user.TransactionPIN == nil || *user.TransactionPIN == "" {
		if pin == "123456" {
			return nil
		}
		return errors.New("PIN transaksi salah. (Gunakan PIN default 123456)")
	}

	err = bcrypt.CompareHashAndPassword([]byte(*user.TransactionPIN), []byte(pin))
	if err != nil {
		return errors.New("PIN transaksi yang Anda masukkan salah")
	}

	return nil
}

// GetBiometricChallenge generates a random challenge for WebAuthn and stores it temporarily
func (uc *authUsecase) GetBiometricChallenge(userID uuid.UUID) (string, error) {
	challengeBytes := make([]byte, 32)
	if _, err := rand.Read(challengeBytes); err != nil {
		return "", fmt.Errorf("failed to generate challenge: %w", err)
	}
	challenge := base64.RawURLEncoding.EncodeToString(challengeBytes)

	uc.biometricMu.Lock()
	uc.biometricChallenges[userID] = biometricChallengeEntry{
		challenge: challenge,
		expiresAt: time.Now().Add(2 * time.Minute),
	}
	uc.biometricMu.Unlock()

	return challenge, nil
}

// RegisterBiometric stores the WebAuthn credential ID and public key for the user
func (uc *authUsecase) RegisterBiometric(userID uuid.UUID, req domain.BiometricRegisterRequest) error {
	user, err := uc.userRepo.GetUserByID(userID)
	if err != nil || user == nil {
		return errors.New("user tidak ditemukan")
	}

	user.BiometricCredentialID = &req.CredentialID
	user.BiometricPublicKey = &req.PublicKeyBase64
	user.BiometricEnabled = true

	return uc.userRepo.UpdateUser(user)
}

type ecdsaSignatureASN1 struct {
	R, S *big.Int
}

func decodeBase64Flex(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if b, err := base64.RawURLEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	if b, err := base64.URLEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	if b, err := base64.RawStdEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	return base64.StdEncoding.DecodeString(s)
}

// VerifyBiometric verifies a WebAuthn assertion using the stored public key and challenge
func (uc *authUsecase) VerifyBiometric(userID uuid.UUID, req domain.BiometricVerifyRequest) error {
	// Check and consume challenge
	uc.biometricMu.Lock()
	entry, exists := uc.biometricChallenges[userID]
	if exists {
		delete(uc.biometricChallenges, userID)
	}
	uc.biometricMu.Unlock()

	if !exists {
		return errors.New("tidak ada challenge aktif. Minta ulang challenge")
	}
	if time.Now().After(entry.expiresAt) {
		return errors.New("challenge sudah expired. Silakan coba lagi")
	}
	if entry.challenge != req.Challenge {
		return errors.New("challenge tidak valid")
	}

	// Retrieve user and their stored public key
	user, err := uc.userRepo.GetUserByID(userID)
	if err != nil || user == nil {
		return errors.New("user tidak ditemukan")
	}
	if !user.BiometricEnabled || user.BiometricCredentialID == nil || user.BiometricPublicKey == nil {
		return errors.New("biometrik belum didaftarkan. Silakan daftarkan fingerprint terlebih dahulu")
	}
	if *user.BiometricCredentialID != req.CredentialID {
		return errors.New("credential ID tidak cocok")
	}

	// Decode the stored public key (DER/SPKI or raw EC public key stored as base64)
	pubKeyBytes, err := decodeBase64Flex(*user.BiometricPublicKey)
	if err != nil {
		return fmt.Errorf("gagal mendecode public key: %w", err)
	}

	var ecPubKey *ecdsa.PublicKey
	pubKeyInterface, err := x509.ParsePKIXPublicKey(pubKeyBytes)
	if err != nil {
		// Fallback: uncompressed EC point (0x04 || X || Y) of 65 bytes
		if len(pubKeyBytes) == 65 && pubKeyBytes[0] == 0x04 {
			curve := elliptic.P256()
			x := new(big.Int).SetBytes(pubKeyBytes[1:33])
			y := new(big.Int).SetBytes(pubKeyBytes[33:])
			ecPubKey = &ecdsa.PublicKey{Curve: curve, X: x, Y: y}
		} else {
			return fmt.Errorf("gagal mem-parse public key: %w", err)
		}
	} else {
		var ok bool
		ecPubKey, ok = pubKeyInterface.(*ecdsa.PublicKey)
		if !ok {
			return errors.New("public key bukan format EC yang valid")
		}
	}

	// Reconstruct the signed data: authenticatorData || SHA256(clientDataJSON)
	authDataBytes, err := decodeBase64Flex(req.AuthenticatorData)
	if err != nil {
		return fmt.Errorf("gagal mendecode authenticatorData: %w", err)
	}
	clientDataBytes, err := decodeBase64Flex(req.ClientDataJSON)
	if err != nil {
		return fmt.Errorf("gagal mendecode clientDataJSON: %w", err)
	}

	// Validate that challenge in clientDataJSON matches our stored challenge
	var clientDataMap map[string]interface{}
	if err := json.Unmarshal(clientDataBytes, &clientDataMap); err == nil {
		if clientChallenge, ok := clientDataMap["challenge"].(string); ok && clientChallenge != "" {
			if clientChallenge != req.Challenge {
				return errors.New("challenge dalam clientDataJSON tidak cocok")
			}
		}
	}

	clientDataHash := sha256.Sum256(clientDataBytes)
	signedData := append(authDataBytes, clientDataHash[:]...)
	signedDataHash := sha256.Sum256(signedData)

	// Decode and verify the ECDSA signature
	sigBytes, err := decodeBase64Flex(req.Signature)
	if err != nil {
		return fmt.Errorf("gagal mendecode signature: %w", err)
	}

	// 1. Try ASN.1 DER decoding (standard WebAuthn browser format: SEQUENCE { r INTEGER, s INTEGER })
	var ecdsaSig ecdsaSignatureASN1
	if _, err := asn1.Unmarshal(sigBytes, &ecdsaSig); err == nil && ecdsaSig.R != nil && ecdsaSig.S != nil {
		if ecdsa.Verify(ecPubKey, signedDataHash[:], ecdsaSig.R, ecdsaSig.S) {
			return nil
		}
	}

	// 2. Fallback: IEEE P1363 format (raw r || s fixed 64 bytes)
	if len(sigBytes) == 64 {
		r := new(big.Int).SetBytes(sigBytes[:32])
		s := new(big.Int).SetBytes(sigBytes[32:])
		if ecdsa.Verify(ecPubKey, signedDataHash[:], r, s) {
			return nil
		}
	} else if len(sigBytes) > 0 {
		half := len(sigBytes) / 2
		r := new(big.Int).SetBytes(sigBytes[:half])
		s := new(big.Int).SetBytes(sigBytes[half:])
		if ecdsa.Verify(ecPubKey, signedDataHash[:], r, s) {
			return nil
		}
	}

	return errors.New("verifikasi biometrik gagal: signature tidak valid")
}

func (uc *authUsecase) DisableBiometric(userID uuid.UUID) error {
	user, err := uc.userRepo.GetUserByID(userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return domain.ErrNotFound
	}

	user.BiometricCredentialID = nil
	user.BiometricPublicKey = nil
	user.BiometricEnabled = false

	if err := uc.userRepo.UpdateUser(user); err != nil {
		return fmt.Errorf("failed to update user biometric settings: %w", err)
	}

	return nil
}

func (uc *authUsecase) GetMe(userID uuid.UUID) (*domain.UserProfileResponse, error) {
	user, err := uc.userRepo.GetUserByID(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return nil, domain.ErrNotFound
	}

	var walletIDStr *string
	wallet, _ := uc.walletRepo.GetWalletByUserID(userID)
	if wallet != nil {
		wStr := wallet.WalletID.String()
		walletIDStr = &wStr
	}

	return &domain.UserProfileResponse{
		UserID:           user.UserID.String(),
		Username:         user.Username,
		Email:            user.Email,
		AvatarURL:        user.AvatarURL,
		IsActive:         user.IsActive,
		TwoFactorEnabled: user.TwoFactorEnabled,
		PINEnabled:       user.PINEnabled,
		BiometricEnabled: user.BiometricEnabled,
		CreatedAt:        user.CreatedAt,
		WalletID:         walletIDStr,
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
