package usecase

import (
	"encoding/hex"
	"testing"
	"time"

	"github.com/bntngridp/ledger-backend/internal/domain"
	pkgcrypto "github.com/bntngridp/ledger-backend/pkg/crypto"
	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
)

func TestSendPaymentEmailOTP_Success(t *testing.T) {
	mockUserRepo := new(MockUserRepository)
	mockWalletRepo := new(MockWalletRepository)
	uc := NewAuthUsecase(mockUserRepo, mockWalletRepo, nil, "MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDE=")

	userID := uuid.New()
	user := &domain.User{
		UserID: userID,
		Email:  "test@example.com",
	}

	mockUserRepo.On("GetUserByID", userID).Return(user, nil)

	err := uc.SendPaymentEmailOTP(userID)
	assert.NoError(t, err)

	mockUserRepo.AssertExpectations(t)
}

func TestVerifyPaymentSecurity_2FADisabled(t *testing.T) {
	mockUserRepo := new(MockUserRepository)
	mockWalletRepo := new(MockWalletRepository)
	uc := NewAuthUsecase(mockUserRepo, mockWalletRepo, nil, "MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDE=")

	userID := uuid.New()
	user := &domain.User{
		UserID:            userID,
		TwoFactorEnabled:  false,
	}

	mockUserRepo.On("GetUserByID", userID).Return(user, nil)

	// When 2FA is disabled, empty codes should succeed
	err := uc.VerifyPaymentSecurity(userID, "", "")
	assert.NoError(t, err)

	mockUserRepo.AssertExpectations(t)
}

func TestVerifyPaymentSecurity_2FAEnabled_Success(t *testing.T) {
	mockUserRepo := new(MockUserRepository)
	mockWalletRepo := new(MockWalletRepository)
	encKeyBase64 := "MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDE="
	uc := NewAuthUsecase(mockUserRepo, mockWalletRepo, nil, encKeyBase64)

	userID := uuid.New()
	key, _ := totp.Generate(totp.GenerateOpts{
		Issuer:      "Ledger",
		AccountName: "test@example.com",
	})
	totpSecret := key.Secret()

	rawKey, _ := hex.DecodeString("3031323334353637383930313233343536373839303132333435363738393031")
	encSecretBytes, _ := pkgcrypto.Encrypt([]byte(totpSecret), rawKey)
	encSecretHex := hex.EncodeToString(encSecretBytes)

	user := &domain.User{
		UserID:           userID,
		Email:            "test@example.com",
		TwoFactorEnabled: true,
		TwoFactorSecret:  &encSecretHex,
	}

	mockUserRepo.On("GetUserByID", userID).Return(user, nil)

	// Generate valid TOTP code
	totpCode, err := totp.GenerateCode(totpSecret, time.Now())
	assert.NoError(t, err)

	// Send payment OTP first to populate emailOTPs map
	err = uc.SendPaymentEmailOTP(userID)
	assert.NoError(t, err)

	// Extract the generated OTP from internal state
	auc := uc.(*authUsecase)
	auc.otpMu.Lock()
	otpCode := auc.emailOTPs[userID].code
	auc.otpMu.Unlock()

	// Verify both 2FA code and Email OTP
	err = uc.VerifyPaymentSecurity(userID, totpCode, otpCode)
	assert.NoError(t, err)

	mockUserRepo.AssertExpectations(t)
}

func TestVerifyPaymentSecurity_2FAEnabled_InvalidTOTP(t *testing.T) {
	mockUserRepo := new(MockUserRepository)
	mockWalletRepo := new(MockWalletRepository)
	encKeyBase64 := "MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDE="
	uc := NewAuthUsecase(mockUserRepo, mockWalletRepo, nil, encKeyBase64)

	userID := uuid.New()
	key, _ := totp.Generate(totp.GenerateOpts{
		Issuer:      "Ledger",
		AccountName: "test@example.com",
	})
	totpSecret := key.Secret()

	rawKey, _ := hex.DecodeString("3031323334353637383930313233343536373839303132333435363738393031")
	encSecretBytes, _ := pkgcrypto.Encrypt([]byte(totpSecret), rawKey)
	encSecretHex := hex.EncodeToString(encSecretBytes)

	user := &domain.User{
		UserID:           userID,
		Email:            "test@example.com",
		TwoFactorEnabled: true,
		TwoFactorSecret:  &encSecretHex,
	}

	mockUserRepo.On("GetUserByID", userID).Return(user, nil)

	_ = uc.SendPaymentEmailOTP(userID)
	auc := uc.(*authUsecase)
	auc.otpMu.Lock()
	otpCode := auc.emailOTPs[userID].code
	auc.otpMu.Unlock()

	// Invalid TOTP code
	err := uc.VerifyPaymentSecurity(userID, "000000", otpCode)
	assert.Error(t, err)
}

func TestVerifyPaymentSecurity_2FAEnabled_InvalidEmailOTP(t *testing.T) {
	mockUserRepo := new(MockUserRepository)
	mockWalletRepo := new(MockWalletRepository)
	encKeyBase64 := "MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDE="
	uc := NewAuthUsecase(mockUserRepo, mockWalletRepo, nil, encKeyBase64)

	userID := uuid.New()
	key, _ := totp.Generate(totp.GenerateOpts{
		Issuer:      "Ledger",
		AccountName: "test@example.com",
	})
	totpSecret := key.Secret()

	rawKey, _ := hex.DecodeString("3031323334353637383930313233343536373839303132333435363738393031")
	encSecretBytes, _ := pkgcrypto.Encrypt([]byte(totpSecret), rawKey)
	encSecretHex := hex.EncodeToString(encSecretBytes)

	user := &domain.User{
		UserID:           userID,
		Email:            "test@example.com",
		TwoFactorEnabled: true,
		TwoFactorSecret:  &encSecretHex,
	}

	mockUserRepo.On("GetUserByID", userID).Return(user, nil)

	totpCode, _ := totp.GenerateCode(totpSecret, time.Now())
	_ = uc.SendPaymentEmailOTP(userID)

	// Invalid Email OTP
	err := uc.VerifyPaymentSecurity(userID, totpCode, "999999")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "kode verifikasi email tidak valid")
}
