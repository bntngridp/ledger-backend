package repository

import (
	"errors"
	"fmt"
	"math"

	"github.com/bntngridp/ledger-backend/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type notificationRepository struct {
	db *gorm.DB
}

// NewNotificationRepository creates a new NotificationRepository backed by postgres/gorm.
func NewNotificationRepository(db *gorm.DB) domain.NotificationRepository {
	return &notificationRepository{db: db}
}

func (r *notificationRepository) CreateNotification(n *domain.Notification) error {
	if n.NotificationID == uuid.Nil {
		n.NotificationID = uuid.New()
	}
	if err := r.db.Create(n).Error; err != nil {
		return fmt.Errorf("create notification: %w", err)
	}
	return nil
}

func (r *notificationRepository) GetNotificationsByUserID(userID uuid.UUID, page, perPage int) ([]domain.Notification, int64, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 50 {
		perPage = 20
	}

	var total int64
	if err := r.db.Model(&domain.Notification{}).
		Where("user_id = ?", userID).
		Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count notifications: %w", err)
	}

	offset := (page - 1) * perPage
	var notifications []domain.Notification
	if err := r.db.
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(perPage).
		Offset(offset).
		Find(&notifications).Error; err != nil {
		return nil, 0, fmt.Errorf("get notifications: %w", err)
	}

	return notifications, total, nil
}

func (r *notificationRepository) MarkAsRead(notificationID, userID uuid.UUID) error {
	result := r.db.Model(&domain.Notification{}).
		Where("notification_id = ? AND user_id = ?", notificationID, userID).
		Update("is_read", true)
	if result.Error != nil {
		return fmt.Errorf("mark as read: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.New("notification not found or does not belong to user")
	}
	return nil
}

func (r *notificationRepository) MarkAllAsRead(userID uuid.UUID) error {
	if err := r.db.Model(&domain.Notification{}).
		Where("user_id = ? AND is_read = false", userID).
		Update("is_read", true).Error; err != nil {
		return fmt.Errorf("mark all as read: %w", err)
	}
	return nil
}

func (r *notificationRepository) GetUnreadCount(userID uuid.UUID) (int64, error) {
	var count int64
	if err := r.db.Model(&domain.Notification{}).
		Where("user_id = ? AND is_read = false", userID).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("get unread count: %w", err)
	}
	return count, nil
}

func (r *notificationRepository) DeleteNotification(notificationID, userID uuid.UUID) error {
	result := r.db.
		Where("notification_id = ? AND user_id = ?", notificationID, userID).
		Delete(&domain.Notification{})
	if result.Error != nil {
		return fmt.Errorf("delete notification: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.New("notification not found or does not belong to user")
	}
	return nil
}

// TotalPages helper (not part of the interface, but used internally in handler mapping)
func TotalPages(total int64, perPage int) int {
	if perPage <= 0 {
		return 0
	}
	return int(math.Ceil(float64(total) / float64(perPage)))
}
