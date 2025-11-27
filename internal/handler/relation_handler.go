package handler

import (
	"errors"
	"net/http"
	"playmatch/backend/internal/database"
	"playmatch/backend/internal/models"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetUserRelationsByID godoc
// @Summary      Get a specific user's relations
// @Description  Fetches a list of a specific user's relations (friends, followers, etc.) based on status and direction.
// @Tags         friendship
// @Produce      json
// @Security     BearerAuth
// @Param        id        path      int     true   "Target User ID"
// @Param        status    query     string  false  "Filter by status (pending, accepted)"
// @Param        direction query     string  false  "Filter by direction (incoming, outgoing)"
// @Success      200       {array}   PublicUserResponse
// @Failure      400       {object}  ErrorResponse
// @Failure      401       {object}  ErrorResponse
// @Router         /users/{id}/relations [get]
func GetUserRelationsByID(c *gin.Context) {
	viewerID, _ := c.Get("userID")
	targetUserIDStr := c.Param("id")
	targetUserID, err := strconv.ParseUint(targetUserIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid target user ID"})
		return
	}

	statusFilter := c.Query("status")
	directionFilter := c.Query("direction")

	var relations []models.UserRelation
	query := database.DB

	switch directionFilter {
	case "incoming":
		query = query.Where("to_user_id = ?", targetUserID)
	case "outgoing":
		query = query.Where("from_user_id = ?", targetUserID)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "A 'direction' query parameter (incoming or outgoing) is required for this endpoint."})
		return
	}

	if statusFilter != "" {
		query = query.Where("status = ?", statusFilter)
	}

	// Preload the user data we need
	if directionFilter == "incoming" {
		query = query.Preload("FromUser")
	} else { // outgoing
		query = query.Preload("ToUser")
	}

	if err := query.Find(&relations).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch relations"})
		return
	}

	// Build the response
	var userResponses []PublicUserResponse
	for _, r := range relations {
		var userInRelation models.User
		if directionFilter == "incoming" {
			userInRelation = r.FromUser
		} else { // outgoing
			userInRelation = r.ToUser
		}

		if userInRelation.ID == 0 {
			continue
		}

		userResponses = append(userResponses, buildPublicUserResponse(userInRelation, viewerID.(uint)))
	}

	c.JSON(http.StatusOK, userResponses)
}

// GetRelations godoc
// @Summary      Get user relations
// @Description  Fetches a list of user relations (friends, followers, etc.) based on status and direction.
// @Tags         friendship
// @Produce      json
// @Security     BearerAuth
// @Param        status    query     string  false  "Filter by status (pending, accepted)"
// @Param        direction query     string  false  "Filter by direction (incoming, outgoing)"
// @Success      200       {array}   PublicUserResponse
// @Failure      400       {object}  ErrorResponse
// @Failure      401       {object}  ErrorResponse
// @Router         /users/me/relations [get]
func GetRelations(c *gin.Context) {
	viewerID, _ := c.Get("userID")
	statusFilter := c.Query("status")
	directionFilter := c.Query("direction")

	var relations []models.UserRelation
	query := database.DB

	switch directionFilter {
	case "incoming":
		query = query.Where("to_user_id = ?", viewerID)
	case "outgoing":
		query = query.Where("from_user_id = ?", viewerID)
	default:
		// If no direction, it could be either. We handle this by checking both fields.
		query = query.Where("from_user_id = ? OR to_user_id = ?", viewerID, viewerID)
	}

	if statusFilter != "" {
		query = query.Where("status = ?", statusFilter)
	}

	// Preload the user data we need
	if directionFilter == "incoming" {
		query = query.Preload("FromUser")
	} else if directionFilter == "outgoing" {
		query = query.Preload("ToUser")
	} else {
		query = query.Preload("FromUser").Preload("ToUser")
	}

	if err := query.Find(&relations).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch relations"})
		return
	}

	// Build the response
	var userResponses []PublicUserResponse
	for _, r := range relations {
		var targetUser models.User
		// Determine which user in the relation is NOT the viewer
		if r.FromUserID == viewerID.(uint) {
			targetUser = r.ToUser
		} else {
			targetUser = r.FromUser
		}

		// Avoid adding empty users if a preload was missed
		if targetUser.ID == 0 {
			continue
		}

		userResponses = append(userResponses, buildPublicUserResponse(targetUser, viewerID.(uint)))
	}

	c.JSON(http.StatusOK, userResponses)
}

// SendRequest godoc
// @Summary      Send friend request
// @Description  Sends a friend request to another user (subscribes).
// @Tags         friendship
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "Target User ID"
// @Success      201  {object}  map[string]string "{"message": "Request sent successfully"}"
// @Failure      400  {object}  ErrorResponse
// @Failure      401  {object}  ErrorResponse
// @Failure      404  {object}  ErrorResponse "Target user not found"
// @Failure      409  {object}  ErrorResponse "Relation already exists"
// @Failure      500  {object}  ErrorResponse
// @Router       /users/{id}/request [post]
func SendRequest(c *gin.Context) {
	// Logic to send a friend request
	viewerID, _ := c.Get("userID")
	targetUserIDStr := c.Param("id")
	targetUserID, err := strconv.ParseUint(targetUserIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid target user ID"})
		return
	}

	if viewerID.(uint) == uint(targetUserID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot send request to yourself"})
		return
	}

	// Check if relation already exists
	var existingRelation models.UserRelation
	err = database.DB.Where("from_user_id = ? AND to_user_id = ?", viewerID, targetUserID).First(&existingRelation).Error
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusConflict, gin.H{"error": "Relation already exists or another error occurred"})
		return
	}

	// Create new relation
	newRelation := models.UserRelation{
		FromUserID: viewerID.(uint),
		ToUserID:   uint(targetUserID),
		Status:     models.StatusPending,
	}

	if err := database.DB.Create(&newRelation).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create relation"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Request sent successfully"})
}

// AcceptRequest godoc
// @Summary      Accept friend request
// @Description  Accepts a pending friend request from another user.
// @Tags         friendship
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "Requesting User ID"
// @Success      200  {object}  map[string]string "{"message": "Request accepted"}"
// @Failure      400  {object}  ErrorResponse
// @Failure      401  {object}  ErrorResponse
// @Failure      404  {object}  ErrorResponse "Request not found"
// @Failure      500  {object}  ErrorResponse
// @Router       /users/{id}/accept [post]
func AcceptRequest(c *gin.Context) {
	// Logic to accept a friend request
	viewerID, _ := c.Get("userID")
	requestingUserIDStr := c.Param("id")
	requestingUserID, err := strconv.ParseUint(requestingUserIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid requesting user ID"})
		return
	}

	// Find the pending request
	var request models.UserRelation
	err = database.DB.Where("from_user_id = ? AND to_user_id = ? AND status = ?", requestingUserID, viewerID, models.StatusPending).First(&request).Error
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Pending request not found"})
		return
	}

	// Update status to accepted
	if err := database.DB.Model(&request).Update("status", models.StatusAccepted).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to accept request"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Request accepted"})
}

// DeclineRequest godoc
// @Summary      Decline friend request
// @Description  Declines a pending friend request from another user.
// @Tags         friendship
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "Requesting User ID"
// @Success      200  {object}  map[string]string "{"message": "Request declined"}"
// @Failure      400  {object}  ErrorResponse
// @Failure      401  {object}  ErrorResponse
// @Failure      404  {object}  ErrorResponse "Request not found"
// @Failure      500  {object}  ErrorResponse
// @Router       /users/{id}/decline [post]
func DeclineRequest(c *gin.Context) {
	// Logic to decline a friend request
	viewerID, _ := c.Get("userID")
	requestingUserIDStr := c.Param("id")
	requestingUserID, err := strconv.ParseUint(requestingUserIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid requesting user ID"})
		return
	}

	// Find and delete the pending request
	result := database.DB.Where("from_user_id = ? AND to_user_id = ? AND status = ?", requestingUserID, viewerID, models.StatusPending).Delete(&models.UserRelation{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decline request"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Pending request not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Request declined"})
}

// RemoveRelation godoc
// @Summary      Remove relation
// @Description  Cancels a sent request, or removes a user from friends (unsubscribes).
// @Tags         friendship
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "Target User ID"
// @Success      200  {object}  map[string]string "{"message": "Relation removed"}"
// @Failure      400  {object}  ErrorResponse
// @Failure      401  {object}  ErrorResponse
// @Failure      404  {object}  ErrorResponse "Relation not found"
// @Failure      500  {object}  ErrorResponse
// @Router       /users/{id}/remove [post]
func RemoveRelation(c *gin.Context) {
	// Logic to cancel a request or remove a friend
	viewerID, _ := c.Get("userID")
	targetUserIDStr := c.Param("id")
	targetUserID, err := strconv.ParseUint(targetUserIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid target user ID"})
		return
	}

	// Find and delete the relation from viewer to target
	result := database.DB.Where("from_user_id = ? AND to_user_id = ?", viewerID, targetUserID).Delete(&models.UserRelation{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove relation"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Relation not found to remove"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Relation removed"})
}
