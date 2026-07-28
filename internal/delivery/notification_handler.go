package delivery

import (
	"net/http"
	"strconv"

	"github.com/bntngridp/ledger-backend/internal/usecase"
	"github.com/bntngridp/ledger-backend/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// NotificationHandler handles HTTP requests for in-app notifications.
type NotificationHandler struct {
	notifUC *usecase.NotificationUsecase
}

// NewNotificationHandler constructs a NotificationHandler.
func NewNotificationHandler(notifUC *usecase.NotificationUsecase) *NotificationHandler {
	return &NotificationHandler{notifUC: notifUC}
}

// getUserID extracts and validates the user_id claim from the Gin context.
func (h *NotificationHandler) getUserID(c *gin.Context) (uuid.UUID, bool) {
	userIDStr, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"status": "error", "message": "unauthorized"})
		return uuid.Nil, false
	}
	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"status": "error", "message": "invalid user token"})
		return uuid.Nil, false
	}
	return userID, true
}

// GetNotifications godoc
// @Summary      List notifications
// @Description  Returns paginated in-app notifications for the authenticated user along with unread count.
// @Tags         notifications
// @Produce      json
// @Security     BearerAuth
// @Param        page     query int false "Page number (default 1)"
// @Param        per_page query int false "Items per page (default 20, max 50)"
// @Success      200 {object} domain.SuccessResponse
// @Failure      401 {object} domain.ErrorResponse
// @Router       /notifications [get]
func (h *NotificationHandler) GetNotifications(c *gin.Context) {
	userID, ok := h.getUserID(c)
	if !ok {
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))

	result, err := h.notifUC.GetNotifications(userID, page, perPage)
	if err != nil {
		response.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Notifications retrieved successfully",
		"data":    result,
	})
}

// GetUnreadCount godoc
// @Summary      Get unread notification count
// @Description  Returns the number of unread notifications for the authenticated user.
// @Tags         notifications
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} domain.SuccessResponse
// @Failure      401 {object} domain.ErrorResponse
// @Router       /notifications/unread-count [get]
func (h *NotificationHandler) GetUnreadCount(c *gin.Context) {
	userID, ok := h.getUserID(c)
	if !ok {
		return
	}

	count, err := h.notifUC.GetUnreadCount(userID)
	if err != nil {
		response.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Unread count retrieved",
		"data":    gin.H{"unread_count": count},
	})
}

// MarkAsRead godoc
// @Summary      Mark notification as read
// @Description  Marks a specific notification as read (must belong to the authenticated user).
// @Tags         notifications
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Notification ID (UUID)"
// @Success      200 {object} domain.SuccessResponse
// @Failure      400 {object} domain.ErrorResponse
// @Failure      401 {object} domain.ErrorResponse
// @Failure      404 {object} domain.ErrorResponse
// @Router       /notifications/{id}/read [patch]
func (h *NotificationHandler) MarkAsRead(c *gin.Context) {
	userID, ok := h.getUserID(c)
	if !ok {
		return
	}

	notifID := c.Param("id")
	if err := h.notifUC.MarkAsRead(notifID, userID); err != nil {
		response.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Notification marked as read",
	})
}

// MarkAllAsRead godoc
// @Summary      Mark all notifications as read
// @Description  Marks all notifications for the authenticated user as read.
// @Tags         notifications
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} domain.SuccessResponse
// @Failure      401 {object} domain.ErrorResponse
// @Router       /notifications/read-all [patch]
func (h *NotificationHandler) MarkAllAsRead(c *gin.Context) {
	userID, ok := h.getUserID(c)
	if !ok {
		return
	}

	if err := h.notifUC.MarkAllAsRead(userID); err != nil {
		response.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "All notifications marked as read",
	})
}

// DeleteNotification godoc
// @Summary      Delete notification
// @Description  Permanently removes a notification (must belong to the authenticated user).
// @Tags         notifications
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Notification ID (UUID)"
// @Success      200 {object} domain.SuccessResponse
// @Failure      401 {object} domain.ErrorResponse
// @Failure      404 {object} domain.ErrorResponse
// @Router       /notifications/{id} [delete]
func (h *NotificationHandler) DeleteNotification(c *gin.Context) {
	userID, ok := h.getUserID(c)
	if !ok {
		return
	}

	notifID := c.Param("id")
	if err := h.notifUC.DeleteNotification(notifID, userID); err != nil {
		response.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Notification deleted",
	})
}
