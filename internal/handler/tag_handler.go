package handler

import (
	"net/http"
	"playmatch/backend/internal/database"
	"playmatch/backend/internal/models"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type TagInput struct {
	Name string `json:"name" binding:"required"`
}

type TagResponse struct {
	ID        uint      `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Name      string    `json:"name"`
}

func newTagResponse(tag models.Tag) TagResponse {
	return TagResponse{
		ID:        tag.ID,
		CreatedAt: tag.CreatedAt,
		UpdatedAt: tag.UpdatedAt,
		Name:      tag.Name,
	}
}

// CreateTag godoc
// @Summary      Create a new tag
// @Description  Creates a new tag for games.
// @Tags         admin-tags
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        input body TagInput true "Tag Info"
// @Success      201  {object}  TagResponse
// @Failure      400  {object}  ErrorResponse
// @Failure      403  {object}  ErrorResponse "Admin access required"
// @Failure      409  {object}  ErrorResponse "Tag already exists"
// @Router       /admin/tags [post]
func CreateTag(c *gin.Context) {
	var input TagInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tag := models.Tag{Name: input.Name}
	if err := database.DB.Create(&tag).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Tag already exists or another error occurred"})
		return
	}

	c.JSON(http.StatusCreated, newTagResponse(tag))
}

// GetTags godoc
// @Summary      Get all tags
// @Description  Retrieves a list of all available tags.
// @Tags         admin-tags
// @Produce      json
// @Security     BearerAuth
// @Success      200  {array}   TagResponse
// @Failure      403  {object}  ErrorResponse "Admin access required"
// @Router       /admin/tags [get]
func GetTags(c *gin.Context) {
	var tags []models.Tag
	database.DB.Find(&tags)

	var response []TagResponse
	for _, tag := range tags {
		response = append(response, newTagResponse(tag))
	}
	c.JSON(http.StatusOK, response)
}

// UpdateTag godoc
// @Summary      Update a tag
// @Description  Updates the name of an existing tag.
// @Tags         admin-tags
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int      true  "Tag ID"
// @Param        input body TagInput true "New Tag Info"
// @Success      200  {object}  TagResponse
// @Failure      400  {object}  ErrorResponse
// @Failure      403  {object}  ErrorResponse "Admin access required"
// @Failure      404  {object}  ErrorResponse "Tag not found"
// @Router       /admin/tags/{id} [put]
func UpdateTag(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	var input TagInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var tag models.Tag
	if err := database.DB.First(&tag, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tag not found"})
		return
	}

	database.DB.Model(&tag).Update("name", input.Name)
	c.JSON(http.StatusOK, newTagResponse(tag))
}

// DeleteTag godoc
// @Summary      Delete a tag
// @Description  Deletes an existing tag.
// @Tags         admin-tags
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "Tag ID"
// @Success      200  {object}  map[string]string "{"message": "Tag deleted"}"
// @Failure      400  {object}  ErrorResponse
// @Failure      403  {object}  ErrorResponse "Admin access required"
// @Failure      404  {object}  ErrorResponse "Tag not found"
// @Router       /admin/tags/{id} [delete]
func DeleteTag(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	result := database.DB.Delete(&models.Tag{}, id)
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tag not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Tag deleted"})
}
