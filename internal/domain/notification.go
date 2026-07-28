package domain

import (
	"time"

	"github.com/google/uuid"
)

// NotificationType represents the category of an in-app notification.
type NotificationType string

const (
	NotificationTypeTopUp      NotificationType = "topup"
	NotificationTypeTransfer   NotificationType = "transfer"
	NotificationTypeWithdraw   NotificationType = "withdraw"
	NotificationTypeSwap       NotificationType = "swap"
	NotificationTypeSecurity   NotificationType = "security"
	NotificationTypeCrypto     NotificationType = "crypto"
	NotificationTypeSystem     NotificationType = "system"
)

// Notification is the domain entity for in-app notifications stored per user.
type Notification struct {
	NotificationID uuid.UUID        `gorm:"type:uuid;default:uuid_generate_v4();primary_key" json:"notification_id"`
	UserID         uuid.UUID        `gorm:"type:uuid;not null;index"                         json:"user_id"`
	Type           NotificationType `gorm:"type:varchar(50);not null"                        json:"type"`
	Title          string           `gorm:"type:varchar(255);not null"                       json:"title"`
	Body           string           `gorm:"type:text;not null"                               json:"body"`
	IsRead         bool             `gorm:"default:false;not null"                           json:"is_read"`
	CreatedAt      time.Time        `gorm:"autoCreateTime;default:now()"                     json:"created_at"`
}
