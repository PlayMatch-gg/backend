package handler

import (
	"net/http"
	"playmatch/backend/internal/database"
	"playmatch/backend/internal/models"
	"strconv"
	"strings" // Added this import

	"github.com/gin-gonic/gin"
)

// region --- DTOs ---

type GameInput struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	SteamURL    string `json:"steam_url"`
	TagIDs      []uint `json:"tag_ids"` // IDs of the tags to associate with the game
}

type GameResponse struct {
	ID          uint          `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	SteamURL    string        `json:"steam_url"`
	Tags        []TagResponse `json:"tags"`
}

func newGameResponse(game models.Game) GameResponse {
	var tagResponses []TagResponse
	for _, tag := range game.Tags {
		if tag != nil {
			tagResponses = append(tagResponses, newTagResponse(*tag))
		}
	}

	return GameResponse{
		ID:          game.ID,
		Name:        game.Name,
		Description: game.Description,
		SteamURL:    game.SteamURL,
		Tags:        tagResponses,
	}
}

// endregion

// region --- Admin Handlers ---

// CreateGame godoc
// @Summary      Create a new game
// @Description  Creates a new game and associates it with given tags.
// @Tags         admin-games
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        input body GameInput true "Game Info"
// @Success      201  {object}  GameResponse
// @Failure      400  {object}  ErrorResponse
// @Failure      403  {object}  ErrorResponse "Admin access required"
// @Router       /admin/games [post]
func CreateGame(c *gin.Context) {
	var input GameInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find tags by IDs
	var tags []*models.Tag
	if len(input.TagIDs) > 0 {
		database.DB.Find(&tags, input.TagIDs)
	}

	game := models.Game{
		Name:        input.Name,
		Description: input.Description,
		SteamURL:    input.SteamURL,
		Tags:        tags,
	}

	if err := database.DB.Create(&game).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create game"})
		return
	}

	c.JSON(http.StatusCreated, newGameResponse(game))
}

// UpdateGame godoc
// @Summary      Update a game
// @Description  Updates a game's details and replaces its tags.
// @Tags         admin-games
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path      int       true  "Game ID"
// @Param        input body      GameInput true  "New Game Info"
// @Success      200   {object}  GameResponse
// @Failure      400   {object}  ErrorResponse
// @Failure      403   {object}  ErrorResponse "Admin access required"
// @Failure      404   {object}  ErrorResponse "Game not found"
// @Router       /admin/games/{id} [put]
func UpdateGame(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	var game models.Game
	if err := database.DB.First(&game, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Game not found"})
		return
	}

	var input GameInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// Find new tags
	var tags []*models.Tag
	if len(input.TagIDs) > 0 {
		database.DB.Find(&tags, input.TagIDs)
	}
	
	// Update game fields
	game.Name = input.Name
	game.Description = input.Description
	game.SteamURL = input.SteamURL

	// Replace association
	if err := database.DB.Model(&game).Association("Tags").Replace(tags); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update tags for game"})
		return
	}
	
	// Save the updated game model itself
	database.DB.Save(&game)
	
	// Preload tags for the response
	database.DB.Preload("Tags").First(&game, id)

	c.JSON(http.StatusOK, newGameResponse(game))
}


// DeleteGame godoc
// @Summary      Delete a game
// @Description  Deletes an existing game.
// @Tags         admin-games
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "Game ID"
// @Success      200 {object} map[string]string "{"message": "Game deleted"}"
// @Failure      400 {object} ErrorResponse
// @Failure      403 {object} ErrorResponse "Admin access required"
// @Failure      404 {object} ErrorResponse "Game not found"
// @Router       /admin/games/{id} [delete]
func DeleteGame(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	
	result := database.DB.Select("Tags").Delete(&models.Game{}, id)
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Game not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Game deleted"})
}


// endregion

// region --- Public Handlers ---

// GetGameByID godoc
// @Summary      Get a single game by ID
// @Description  Retrieves details for a single game, including its tags.
// @Tags         games
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "Game ID"
// @Success      200 {object} GameResponse
// @Failure      404 {object} ErrorResponse "Game not found"
// @Router       /games/{id} [get]
func GetGameByID(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	
	var game models.Game
	if err := database.DB.Preload("Tags").First(&game, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Game not found"})
		return
	}

	c.JSON(http.StatusOK, newGameResponse(game))
}

// GetGames godoc
// @Summary      Get a list of games
// @Description  Retrieves a paginated list of games, with optional filtering by name and tags.
// @Tags         games
// @Produce      json
// @Security     BearerAuth
// @Param        q       query     string  false  "Search query for game name"
// @Param        tag_ids query     []int   false  "Comma-separated list of Tag IDs" collectionFormat:"csv"
// @Param        page    query     int     false  "Page number" default(1)
// @Param        limit   query     int     false  "Items per page" default(10)
// @Success      200 {array} GameResponse
// @Router       /games [get]
func GetGames(c *gin.Context) {
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
	searchQuery := c.Query("q")
	tagIDsStr := c.Query("tag_ids")

	var games []models.Game
	dbQuery := database.DB.Model(&models.Game{})

	// Filter by name
	if searchQuery != "" {
		dbQuery = dbQuery.Where("name ILIKE ?", "%"+searchQuery+"%")
	}

	// Filter by tags
	if tagIDsStr != "" {
		tagIDs := []uint{}
		for _, s := range splitCommaSeparated(tagIDsStr) {
			if id, parseErr := strconv.ParseUint(s, 10, 32); parseErr == nil {
				tagIDs = append(tagIDs, uint(id))
			}
		}

		if len(tagIDs) > 0 {
			// Find games that have at least one of the specified tags
			dbQuery = dbQuery.Joins("JOIN game_tags gt ON gt.game_id = games.id").
				Where("gt.tag_id IN (?)", tagIDs).
				Group("games.id") // Group by game.id to avoid duplicate games if they have multiple matching tags
		}
	}

	dbQuery = dbQuery.Preload("Tags").Offset(offset).Limit(limit).Find(&games)
	
	var response []GameResponse
	for _, game := range games {
		response = append(response, newGameResponse(game))
	}

	c.JSON(http.StatusOK, response)
}

// Helper to split comma-separated strings
func splitCommaSeparated(s string) []string {
	var result []string
	parts := strings.Split(s, ",")
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
// endregion
