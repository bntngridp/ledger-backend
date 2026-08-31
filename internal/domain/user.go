package domain

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	UserID    uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primary_key" json:"user_id"`
	Username  string    `gorm:"type:varchar(50);uniqueIndex;not null" json:"username"`
	Email     string    `gorm:"type:varchar(100);uniqueIndex;not null" json:"email"`
	Password  *string   `gorm:"type:varchar(255);default:null" json:"-"`
	GoogleID  *string   `gorm:"type:varchar(255);uniqueIndex;default:null" json:"google_id,omitempty"`
	AvatarURL *string   `gorm:"type:text;default:null" json:"avatar_url,omitempty"`
	IsActive         bool      `gorm:"default:true;not null" json:"is_active"`
	TwoFactorSecret        *string   `gorm:"type:text;default:null" json:"-"`
	TwoFactorRecoveryCodes *string   `gorm:"type:text;default:null" json:"-"`
	TwoFactorEnabled       bool      `gorm:"default:false;not null" json:"two_factor_enabled"`
	TransactionPIN         *string   `gorm:"type:varchar(255);default:null" json:"-"`
	PINEnabled             bool      `gorm:"default:false;not null" json:"pin_enabled"`
	BiometricCredentialID  *string   `gorm:"type:text;default:null" json:"-"`
	BiometricPublicKey     *string   `gorm:"type:text;default:null" json:"-"`
	BiometricEnabled       bool      `gorm:"default:false;not null" json:"biometric_enabled"`
	CreatedAt        time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time `gorm:"autoUpdateTime" json:"updated_at"`
	Wallet           *Wallet   `gorm:"foreignKey:UserID" json:"wallet,omitempty"`
}
