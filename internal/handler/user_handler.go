package handler

import (
	"errors"
	"net/http"
	"playmatch/backend/internal/database"
	"playmatch/backend/internal/models"
	"playmatch/backend/pkg/jwt"
	"strconv"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// region --- DTOs ---

// RegisterInput defines the structure for user registration.
type RegisterInput struct {
	Nickname string `json:"nickname" binding:"required" example:"testuser"`
	Email    string `json:"email" binding:"required,email" example:"test@example.com"`
	Password string `json:"password" binding:"required,min=8" example:"password123"`
}

// LoginInput defines the structure for user login.
type LoginInput struct {
	Login    string `json:"login" binding:"required" example:"testuser"`
	Password string `json:"password" binding:"required" example:"password123"`
}

// PublicUserResponse defines the structure for a user's public profile.
type PublicUserResponse struct {
	ID             uint                       `json:"id" example:"1"`
	Nickname       string                     `json:"nickname" example:"testuser"`
	FriendsCount   int64                      `json:"friends_count"`
	FollowersCount int64                      `json:"followers_count"`
	FollowingCount int64                      `json:"following_count"`
	RelationToMe   *models.FriendshipStatus `json:"relation_to_me,omitempty"`
	MeToRelation   *models.FriendshipStatus `json:"me_to_relation,omitempty"`
}

// PrivateUserResponse defines the structure for the authenticated user's own profile.
type PrivateUserResponse struct {
	ID             uint   `json:"id" example:"1"`
	Nickname       string `json:"nickname" example:"testuser"`
	Email          string `json:"email" example:"test@example.com"`
	FriendsCount   int64  `json:"friends_count"`
	FollowersCount int64  `json:"followers_count"`
	FollowingCount int64  `json:"following_count"`
}

// ErrorResponse represents a generic error response.
type ErrorResponse struct {
	Error string `json:"error" example:"An error message"`
}

// PaginatedUserResponse defines the structure for a paginated list of users.
type PaginatedUserResponse struct {
	Data []PublicUserResponse `json:"data"`
	Meta PaginationMeta       `json:"meta"`
}

// PaginationMeta defines the structure for pagination metadata.
type PaginationMeta struct {
	TotalItems  int64 `json:"total_items"`
	TotalPages  int   `json:"total_pages"`
	CurrentPage int   `json:"current_page"`
	PageSize    int   `json:"page_size"`
}

// endregion

// region --- Auth Handlers ---

// RegisterUser godoc
// @Summary      Register a new user
// @Description  Creates a new user and returns an authentication token.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        input body RegisterInput true "Registration Info"
// @Success      201  {object}  map[string]string "{"token": "..."}"
// @Failure      400  {object}  ErrorResponse
// @Failure      409  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /auth/register [post]
func RegisterUser(c *gin.Context) {
	var input RegisterInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var existingUser models.User
	if err := database.DB.Where("nickname = ? OR email = ?", input.Nickname, input.Email).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Nickname or email already exists"})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	user := models.User{
		Nickname:     input.Nickname,
		Email:        input.Email,
		PasswordHash: string(hashedPassword),
	}
	if err := database.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	token, err := jwt.GenerateToken(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"token": token})
}

// LoginUser godoc
// @Summary      Log in a user
// @Description  Authenticates a user with nickname/email and password, and returns a new token.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        input body LoginInput true "Login Info"
// @Success      200  {object}  map[string]string "{"token": "..."}"
// @Failure      400  {object}  ErrorResponse "Invalid input"
// @Failure      401  {object}  ErrorResponse "Invalid credentials"
// @Failure      404  {object}  ErrorResponse "User not found"
// @Failure      500  {object}  ErrorResponse "Internal server error"
// @Router       /auth/login [post]
func LoginUser(c *gin.Context) {
	var input LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if err := database.DB.Where("nickname = ? OR email = ?", input.Login, input.Login).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	token, err := jwt.GenerateToken(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token})
}

// endregion

// region --- User Handlers ---

// SearchUsers godoc
// @Summary      Search for users
// @Description  Searches for users by nickname with pagination.
// @Tags         users
// @Produce      json
// @Security     BearerAuth
// @Param        q     query     string  false  "Search query for nickname"
// @Param        page  query     int     false  "Page number" default(1)
// @Param        limit query     int     false  "Items per page" default(10)
// @Success      200   {object}  PaginatedUserResponse
// @Failure      400   {object}  ErrorResponse
// @Failure      401   {object}  ErrorResponse
// @Router       /users [get]
func SearchUsers(c *gin.Context) {
	viewerID, _ := c.Get("userID")
	searchQuery := c.Query("q")

	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if err != nil || limit < 1 {
		limit = 10
	}
	if limit > 100 {
		limit = 100 // Max limit
	}

	offset := (page - 1) * limit

	var users []models.User
	var totalItems int64

	query := database.DB.Model(&models.User{})
	if searchQuery != "" {
		query = query.Where("nickname ILIKE ?", "%"+searchQuery+"%")
	}

	// Get total count before pagination
	if err := query.Count(&totalItems).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count users"})
		return
	}

	// Get paginated users
	if err := query.Limit(limit).Offset(offset).Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve users"})
		return
	}

	// Build response
	userResponses := make([]PublicUserResponse, len(users))
	for i, user := range users {
		// Don't show the viewer in the search results
		if user.ID == viewerID.(uint) {
			continue
		}
		userResponses[i] = buildPublicUserResponse(user, viewerID.(uint))
	}
	
	// Filter out the empty entry for the viewer
	finalResponses := []PublicUserResponse{}
	for _, res := range userResponses {
		if res.ID != 0 {
			finalResponses = append(finalResponses, res)
		}
	}


	c.JSON(http.StatusOK, PaginatedUserResponse{
		Data: finalResponses,
		Meta: PaginationMeta{
			TotalItems:  totalItems,
			TotalPages:  (int(totalItems) + limit - 1) / limit,
			CurrentPage: page,
			PageSize:    limit,
		},
	})
}

// GetUserByID godoc
// @Summary      Get user by ID
// @Description  Retrieves the public profile for a specific user by their ID, including relationship data.
// @Tags         users
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "User ID"
// @Success      200  {object}  PublicUserResponse
// @Failure      400  {object}  ErrorResponse
// @Failure      401  {object}  ErrorResponse
// @Failure      404  {object}  ErrorResponse
// @Router       /users/{id} [get]
func GetUserByID(c *gin.Context) {
	viewerID, _ := c.Get("userID")
	targetUserIDStr := c.Param("id")
	targetUserID, err := strconv.ParseUint(targetUserIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// If target is the same as viewer, redirect to /me
	if viewerID.(uint) == uint(targetUserID) {
		GetMe(c)
		return
	}

	var targetUser models.User
	if err := database.DB.First(&targetUser, uint(targetUserID)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	response := buildPublicUserResponse(targetUser, viewerID.(uint))
	c.JSON(http.StatusOK, response)
}

// GetMe godoc
// @Summary      Get current user's info
// @Description  Retrieves the private profile for the currently authenticated user.
// @Tags         users
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  PrivateUserResponse
// @Failure      401  {object}  ErrorResponse
// @Failure      404  {object}  ErrorResponse
// @Router       /users/me [get]
func GetMe(c *gin.Context) {
	viewerID, _ := c.Get("userID")

	var user models.User
	if err := database.DB.First(&user, viewerID.(uint)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	response := buildPrivateUserResponse(user)
	c.JSON(http.StatusOK, response)
}

// endregion

// region --- Helpers ---

func buildPublicUserResponse(targetUser models.User, viewerID uint) PublicUserResponse {
	// These counts can be optimized later if performance is an issue
	var friendsCount, followersCount, followingCount int64
	database.DB.Model(&models.UserRelation{}).Where("to_user_id = ? AND status = ?", targetUser.ID, models.StatusAccepted).Count(&friendsCount)
	database.DB.Model(&models.UserRelation{}).Where("to_user_id = ? AND status = ?", targetUser.ID, models.StatusPending).Count(&followersCount)
	database.DB.Model(&models.UserRelation{}).Where("from_user_id = ? AND status = ?", targetUser.ID, models.StatusPending).Count(&followingCount)

	// Get relationship status between viewer and target
	var relationToMe, meToRelation models.UserRelation
	var relationToMeStatus, meToRelationStatus *models.FriendshipStatus

	err := database.DB.Where("from_user_id = ? AND to_user_id = ?", targetUser.ID, viewerID).First(&relationToMe).Error
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		relationToMeStatus = &relationToMe.Status
	}

	err = database.DB.Where("from_user_id = ? AND to_user_id = ?", viewerID, targetUser.ID).First(&meToRelation).Error
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		meToRelationStatus = &meToRelation.Status
	}

	return PublicUserResponse{
		ID:             targetUser.ID,
		Nickname:       targetUser.Nickname,
		FriendsCount:   friendsCount,
		FollowersCount: followersCount,
		FollowingCount: followingCount,
		RelationToMe:   relationToMeStatus,
		MeToRelation:   meToRelationStatus,
	}
}

func buildPrivateUserResponse(user models.User) PrivateUserResponse {
	var friendsCount, followersCount, followingCount int64
	database.DB.Model(&models.UserRelation{}).Where("to_user_id = ? AND status = ?", user.ID, models.StatusAccepted).Count(&friendsCount)
	database.DB.Model(&models.UserRelation{}).Where("to_user_id = ? AND status = ?", user.ID, models.StatusPending).Count(&followersCount)
	database.DB.Model(&models.UserRelation{}).Where("from_user_id = ? AND status = ?", user.ID, models.StatusPending).Count(&followingCount)

	return PrivateUserResponse{
		ID:             user.ID,
		Nickname:       user.Nickname,
		Email:          user.Email,
		FriendsCount:   friendsCount,
		FollowersCount: followersCount,
		FollowingCount: followingCount,
	}
}

// endregion
