package usecase

import (
	"fmt"
	"math"

	"github.com/bntngridp/ledger-backend/internal/domain"
	"github.com/google/uuid"
)

// NotificationUsecase encapsulates business logic for in-app notifications.
type NotificationUsecase struct {
	notifRepo domain.NotificationRepository
}

// NewNotificationUsecase creates a NotificationUsecase.
func NewNotificationUsecase(notifRepo domain.NotificationRepository) *NotificationUsecase {
	return &NotificationUsecase{notifRepo: notifRepo}
}

// CreateNotification creates and persists a new notification for a user.
func (uc *NotificationUsecase) CreateNotification(userID uuid.UUID, notifType domain.NotificationType, title, body string) error {
	n := &domain.Notification{
		UserID: userID,
		Type:   notifType,
		Title:  title,
		Body:   body,
	}
	if err := uc.notifRepo.CreateNotification(n); err != nil {
		return fmt.Errorf("create notification: %w", err)
	}
	return nil
}

// GetNotifications returns paginated notifications with unread count for a user.
func (uc *NotificationUsecase) GetNotifications(userID uuid.UUID, page, perPage int) (*domain.NotificationListResponse, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 50 {
		perPage = 20
	}

	notifications, total, err := uc.notifRepo.GetNotificationsByUserID(userID, page, perPage)
	if err != nil {
		return nil, fmt.Errorf("get notifications: %w", err)
	}

	unreadCount, err := uc.notifRepo.GetUnreadCount(userID)
	if err != nil {
		return nil, fmt.Errorf("get unread count: %w", err)
	}

	items := make([]domain.NotificationItem, 0, len(notifications))
	for _, n := range notifications {
		items = append(items, domain.NotificationItem{
			NotificationID: n.NotificationID.String(),
			Type:           string(n.Type),
			Title:          n.Title,
			Body:           n.Body,
			IsRead:         n.IsRead,
			CreatedAt:      n.CreatedAt,
		})
	}

	totalPages := int(math.Ceil(float64(total) / float64(perPage)))

	return &domain.NotificationListResponse{
		Notifications: items,
		Meta: domain.PaginationMeta{
			Page:       page,
			PerPage:    perPage,
			Total:      total,
			TotalPages: totalPages,
		},
		UnreadCount: unreadCount,
	}, nil
}

// MarkAsRead marks a single notification as read, ensuring ownership.
func (uc *NotificationUsecase) MarkAsRead(notificationIDStr string, userID uuid.UUID) error {
	notifID, err := uuid.Parse(notificationIDStr)
	if err != nil {
		return domain.ErrNotFound
	}
	if err := uc.notifRepo.MarkAsRead(notifID, userID); err != nil {
		return fmt.Errorf("mark as read: %w", err)
	}
	return nil
}

// MarkAllAsRead marks all notifications as read for the given user.
func (uc *NotificationUsecase) MarkAllAsRead(userID uuid.UUID) error {
	if err := uc.notifRepo.MarkAllAsRead(userID); err != nil {
		return fmt.Errorf("mark all as read: %w", err)
	}
	return nil
}

// GetUnreadCount returns the number of unread notifications for the user.
func (uc *NotificationUsecase) GetUnreadCount(userID uuid.UUID) (int64, error) {
	count, err := uc.notifRepo.GetUnreadCount(userID)
	if err != nil {
		return 0, fmt.Errorf("get unread count: %w", err)
	}
	return count, nil
}

// DeleteNotification removes a notification, ensuring the caller is the owner.
func (uc *NotificationUsecase) DeleteNotification(notificationIDStr string, userID uuid.UUID) error {
	notifID, err := uuid.Parse(notificationIDStr)
	if err != nil {
		return domain.ErrNotFound
	}
	if err := uc.notifRepo.DeleteNotification(notifID, userID); err != nil {
		return fmt.Errorf("delete notification: %w", err)
	}
	return nil
}

// DeleteAllNotifications removes all notifications for the user.
func (uc *NotificationUsecase) DeleteAllNotifications(userID uuid.UUID) error {
	if err := uc.notifRepo.DeleteAllNotifications(userID); err != nil {
		return fmt.Errorf("delete all notifications: %w", err)
	}
	return nil
}

// DeleteBulkNotifications removes multiple notifications given their string IDs for the user.
func (uc *NotificationUsecase) DeleteBulkNotifications(notificationIDStrs []string, userID uuid.UUID) error {
	uuids := make([]uuid.UUID, 0, len(notificationIDStrs))
	for _, idStr := range notificationIDStrs {
		if id, err := uuid.Parse(idStr); err == nil {
			uuids = append(uuids, id)
		}
	}
	if len(uuids) == 0 {
		return nil
	}
	if err := uc.notifRepo.DeleteMultipleNotifications(uuids, userID); err != nil {
		return fmt.Errorf("delete multiple notifications: %w", err)
	}
	return nil
}
