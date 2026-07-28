package usecase

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bntngridp/ledger-backend/internal/domain"
	pkgcrypto "github.com/bntngridp/ledger-backend/pkg/crypto"
	"github.com/bntngridp/ledger-backend/pkg/email"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
)

func (uc *authUsecase) Generate2FASecret(userID uuid.UUID) (*domain.Enable2FAResponse, error) {
	user, err := uc.userRepo.GetUserByID(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return nil, domain.ErrNotFound
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "LedgerHybridWallet",
		AccountName: user.Email,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate TOTP key: %w", err)
	}

	secret := key.Secret()

	encrypted, err := pkgcrypto.Encrypt([]byte(secret), uc.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt TOTP secret: %w", err)
	}
	encryptedHex := hex.EncodeToString(encrypted)

	if err := uc.userRepo.Update2FA(userID, &encryptedHex, false); err != nil {
		return nil, fmt.Errorf("failed to save 2FA secret: %w", err)
	}

	return &domain.Enable2FAResponse{
		Secret:    secret,
		QRCodeURL: key.URL(),
	}, nil
}

func generateRecoveryCodes() ([]string, string) {
	codes := make([]string, 16)
	const hexChars = "0123456789abcdef"
	for i := 0; i < 16; i++ {
		b1 := make([]byte, 5)
		rand.Read(b1)
		b2 := make([]byte, 5)
		rand.Read(b2)
		part1 := make([]byte, 5)
		part2 := make([]byte, 5)
		for j := 0; j < 5; j++ {
			part1[j] = hexChars[int(b1[j])%len(hexChars)]
			part2[j] = hexChars[int(b2[j])%len(hexChars)]
		}
		codes[i] = fmt.Sprintf("%s-%s", string(part1), string(part2))
	}
	return codes, strings.Join(codes, ",")
}

func (uc *authUsecase) Enable2FA(userID uuid.UUID, code string) ([]string, error) {
	user, err := uc.userRepo.GetUserByID(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return nil, domain.ErrNotFound
	}

	if user.TwoFactorSecret == nil {
		return nil, errors.New("2FA secret has not been generated")
	}

	encBytes, err := hex.DecodeString(*user.TwoFactorSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to decode encrypted secret: %w", err)
	}

	decrypted, err := pkgcrypto.Decrypt(encBytes, uc.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt secret: %w", err)
	}
	secretStr := string(decrypted)

	if !totp.Validate(code, secretStr) {
		return nil, domain.ErrInvalid2FACode
	}

	// Generate 8 Backup Recovery Codes
	rawCodes, codesStr := generateRecoveryCodes()

	encCodes, err := pkgcrypto.Encrypt([]byte(codesStr), uc.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt recovery codes: %w", err)
	}
	encCodesHex := hex.EncodeToString(encCodes)

	if err := uc.userRepo.Update2FAWithRecoveryCodes(userID, user.TwoFactorSecret, &encCodesHex, true); err != nil {
		return nil, fmt.Errorf("failed to enable 2FA: %w", err)
	}

	return rawCodes, nil
}

func (uc *authUsecase) Send2FAEmailOTP(userID uuid.UUID) error {
	user, err := uc.userRepo.GetUserByID(userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return domain.ErrNotFound
	}

	otpCode := email.GenerateNumericOTP()

	uc.otpMu.Lock()
	uc.emailOTPs[userID] = emailOTPEntry{
		code:      otpCode,
		expiresAt: time.Now().Add(5 * time.Minute),
	}
	uc.otpMu.Unlock()

	uc.emailService.SendOTPEmailAsync(user.Email, otpCode, "Menonaktifkan 2FA / Pemulihan Akun")
	return nil
}

func (uc *authUsecase) Disable2FA(userID uuid.UUID, req domain.Disable2FARequest) error {
	user, err := uc.userRepo.GetUserByID(userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return domain.ErrNotFound
	}

	if !user.TwoFactorEnabled {
		return errors.New("2FA is not enabled")
	}

	verified := false

	// Method 1: Recovery Code (xxxxx-xxxxx)
	if req.RecoveryCode != "" {
		if user.TwoFactorRecoveryCodes != nil {
			encBytes, err := hex.DecodeString(*user.TwoFactorRecoveryCodes)
			if err == nil {
				decrypted, err := pkgcrypto.Decrypt(encBytes, uc.encryptionKey)
				if err == nil {
					savedCodes := strings.Split(string(decrypted), ",")
					cleanedInput := strings.TrimSpace(req.RecoveryCode)
					for _, sc := range savedCodes {
						if strings.EqualFold(sc, cleanedInput) {
							verified = true
							break
						}
					}
				}
			}
		}
		if !verified {
			return errors.New("kode pemulihan (recovery code) tidak valid")
		}
	}

	// Method 2: 6-Digit TOTP Code (Standard)
	if !verified && req.Code != "" {
		if user.TwoFactorSecret == nil {
			return errors.New("2FA secret missing")
		}

		encBytes, err := hex.DecodeString(*user.TwoFactorSecret)
		if err != nil {
			return fmt.Errorf("failed to decode encrypted secret: %w", err)
		}

		decrypted, err := pkgcrypto.Decrypt(encBytes, uc.encryptionKey)
		if err != nil {
			return fmt.Errorf("failed to decrypt secret: %w", err)
		}
		secretStr := string(decrypted)

		if totp.Validate(req.Code, secretStr) {
			verified = true
		} else {
			return domain.ErrInvalid2FACode
		}
	}

	if !verified {
		return errors.New("silakan masukkan Kode OTP Authenticator atau Kode Pemulihan (Recovery Code)")
	}

	if err := uc.userRepo.Update2FAWithRecoveryCodes(userID, nil, nil, false); err != nil {
		return fmt.Errorf("failed to disable 2FA: %w", err)
	}

	return nil
}

func (uc *authUsecase) Verify2FALogin(preAuthToken, code, jwtSecret string, expiryHours int) (*domain.LoginResponse, error) {
	token, err := jwt.Parse(preAuthToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(jwtSecret), nil
	})

	if err != nil || !token.Valid {
		return nil, domain.ErrUnauthorized
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	preAuthClaim, ok := claims["pre_auth"].(bool)
	if !ok || !preAuthClaim {
		return nil, domain.ErrUnauthorized
	}

	userIDStr, ok := claims["user_id"].(string)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, domain.ErrUnauthorized
	}

	user, err := uc.userRepo.GetUserByID(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return nil, domain.ErrNotFound
	}

	if user.TwoFactorSecret == nil {
		return nil, errors.New("2FA secret missing")
	}

	encBytes, err := hex.DecodeString(*user.TwoFactorSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to decode encrypted secret: %w", err)
	}

	decrypted, err := pkgcrypto.Decrypt(encBytes, uc.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt secret: %w", err)
	}
	secretStr := string(decrypted)

	if !totp.Validate(code, secretStr) {
		return nil, domain.ErrInvalid2FACode
	}

	return uc.generateJWTResponse(user, jwtSecret, expiryHours)
}

func (uc *authUsecase) generatePreAuthToken(user *domain.User, jwtSecret string) (string, error) {
	expiry := time.Now().Add(5 * time.Minute) // 5 minutes challenge timeout

	claims := jwt.MapClaims{
		"user_id":  user.UserID.String(),
		"pre_auth": true,
		"exp":      expiry.Unix(),
		"iat":      time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		return "", fmt.Errorf("failed to sign pre-auth token: %w", err)
	}
	return tokenStr, nil
}

func (uc *authUsecase) Verify2FACode(userID uuid.UUID, code string) error {
	user, err := uc.userRepo.GetUserByID(userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return domain.ErrNotFound
	}

	if !user.TwoFactorEnabled {
		return nil
	}

	if code == "" {
		return domain.Err2FARequired
	}

	if user.TwoFactorSecret == nil {
		return errors.New("2FA secret missing")
	}

	encBytes, err := hex.DecodeString(*user.TwoFactorSecret)
	if err != nil {
		return fmt.Errorf("failed to decode encrypted secret: %w", err)
	}

	decrypted, err := pkgcrypto.Decrypt(encBytes, uc.encryptionKey)
	if err != nil {
		return fmt.Errorf("failed to decrypt secret: %w", err)
	}
	secretStr := string(decrypted)

	if !totp.Validate(code, secretStr) {
		return domain.ErrInvalid2FACode
	}

	return nil
}

func (uc *authUsecase) SendChangePasswordEmailOTP(userID uuid.UUID) error {
	user, err := uc.userRepo.GetUserByID(userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return domain.ErrNotFound
	}

	otpCode := email.GenerateNumericOTP()

	uc.otpMu.Lock()
	uc.emailOTPs[userID] = emailOTPEntry{
		code:      otpCode,
		expiresAt: time.Now().Add(5 * time.Minute),
	}
	uc.otpMu.Unlock()

	if uc.emailService != nil {
		uc.emailService.SendOTPEmailAsync(user.Email, otpCode, "Verifikasi Ubah Kata Sandi Ledger")
	}
	return nil
}

func (uc *authUsecase) ChangePassword(userID uuid.UUID, req domain.ChangePasswordRequest) error {
	user, err := uc.userRepo.GetUserByID(userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return domain.ErrNotFound
	}

	if user.Password == nil {
		return errors.New("akun terhubung via OAuth Google, tidak memiliki kata sandi lama")
	}

	// 1. Verify current/old password
	if err := bcrypt.CompareHashAndPassword([]byte(*user.Password), []byte(req.OldPassword)); err != nil {
		return errors.New("kata sandi lama tidak sesuai")
	}

	// 2. Verify Email OTP
	uc.otpMu.Lock()
	otpEntry, exists := uc.emailOTPs[userID]
	if exists {
		delete(uc.emailOTPs, userID)
	}
	uc.otpMu.Unlock()

	if !exists || time.Now().After(otpEntry.expiresAt) || otpEntry.code != req.EmailOTP {
		return errors.New("kode OTP email tidak valid atau sudah kadaluwarsa")
	}

	// 3. Verify 2FA TOTP code if 2FA is enabled
	if user.TwoFactorEnabled {
		if req.TwoFactorCode == "" {
			return errors.New("kode 2FA Authenticator wajib diisi karena 2FA aktif pada akun Anda")
		}
		if user.TwoFactorSecret == nil {
			return errors.New("2FA secret missing")
		}

		encBytes, err := hex.DecodeString(*user.TwoFactorSecret)
		if err != nil {
			return fmt.Errorf("failed to decode encrypted secret: %w", err)
		}

		decrypted, err := pkgcrypto.Decrypt(encBytes, uc.encryptionKey)
		if err != nil {
			return fmt.Errorf("failed to decrypt secret: %w", err)
		}
		secretStr := string(decrypted)

		if !totp.Validate(req.TwoFactorCode, secretStr) {
			return domain.ErrInvalid2FACode
		}
	}

	// 4. Hash new password & update
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash new password: %w", err)
	}

	hashedPasswordStr := string(hashedPassword)
	user.Password = &hashedPasswordStr

	if err := uc.userRepo.UpdateUser(user); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	return nil
}
