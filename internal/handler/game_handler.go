package handler

import (
	"net/http"
	"playmatch/backend/internal/database"
	"playmatch/backend/internal/models"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// ... (keep existing DTOs)

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
	IsFavorite  bool          `json:"is_favorite"`
	Tags        []TagResponse `json:"tags"`
}

func newGameResponse(game models.Game, favoriteIDs map[uint]bool) GameResponse {
	var tagResponses []TagResponse
	for _, tag := range game.Tags {
		if tag != nil {
			tagResponses = append(tagResponses, newTagResponse(*tag))
		}
	}

	_, isFav := favoriteIDs[game.ID]

	return GameResponse{
		ID:          game.ID,
		Name:        game.Name,
		Description: game.Description,
		SteamURL:    game.SteamURL,
		IsFavorite:  isFav,
		Tags:        tagResponses,
	}
}

// PaginatedGameResponse defines the structure for a paginated list of games.
type PaginatedGameResponse struct {
	Data []GameResponse `json:"data"`
	Meta PaginationMeta `json:"meta"`
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

	c.JSON(http.StatusCreated, newGameResponse(game, nil)) // No favorites context on create
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

	c.JSON(http.StatusOK, newGameResponse(game, nil)) // No favorites context on update
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

// ToggleFavoriteGame godoc
// @Summary      Toggle a game in favorites
// @Description  Adds or removes a game from the user's favorites list.
// @Tags         games
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "Game ID"
// @Success      200 {object} map[string]bool "{"is_favorite": true}"
// @Failure      401 {object} ErrorResponse
// @Failure      404 {object} ErrorResponse "User or game not found"
// @Failure      500 {object} ErrorResponse "Failed to update favorites"
// @Router       /games/{id}/favorite [post]
func ToggleFavoriteGame(c *gin.Context) {
	userID, _ := c.Get("userID")
	gameID, _ := strconv.Atoi(c.Param("id"))

	var user models.User
	// Eagerly load just the one favorite game we care about
	if err := database.DB.Preload("FavoriteGames", "id = ?", gameID).First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var game models.Game
	if err := database.DB.First(&game, gameID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Game not found"})
		return
	}

	association := database.DB.Model(&user).Association("FavoriteGames")

	// If the preload found the game, it's already a favorite
	if len(user.FavoriteGames) > 0 {
		if err := association.Delete(&game); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove from favorites"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"is_favorite": false})
	} else {
		if err := association.Append(&game); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add to favorites"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"is_favorite": true})
	}
}

// GetGameByID godoc
// @Summary      Get a single game by ID
// @Description  Retrieves details for a single game, including its tags and favorite status.
// @Tags         games
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "Game ID"
// @Success      200 {object} GameResponse
// @Failure      404 {object} ErrorResponse "Game not found"
// @Router       /games/{id} [get]
func GetGameByID(c *gin.Context) {
	userID, _ := c.Get("userID")
	id, _ := strconv.Atoi(c.Param("id"))

	var game models.Game
	if err := database.DB.Preload("Tags").First(&game, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Game not found"})
		return
	}

	// Check if this game is a favorite for the current user
	var user models.User
	database.DB.Preload("FavoriteGames", "id = ?", id).First(&user, userID)

	favoriteIDs := make(map[uint]bool)
	if len(user.FavoriteGames) > 0 {
		favoriteIDs[uint(id)] = true
	}

	c.JSON(http.StatusOK, newGameResponse(game, favoriteIDs))
}

// GetGames godoc
// @Summary      Get a list of games
// @Description  Retrieves a paginated list of games, with optional filtering by name, tags, and favorites.
// @Tags         games
// @Produce      json
// @Security     BearerAuth
// @Param        q       query     string  false  "Search query for game name"
// @Param        tag_ids query     string  false  "Comma-separated list of Tag IDs"
// @Param        favorites_only query bool false "Return only favorite games"
// @Param        page    query     int     false  "Page number" default(1)
// @Param        limit   query     int     false  "Items per page" default(10)
// @Success      200 {object} PaginatedGameResponse
// @Router       /games [get]
func GetGames(c *gin.Context) {
	userID, _ := c.Get("userID")
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
	favoritesOnly, _ := strconv.ParseBool(c.Query("favorites_only"))

	// Get user's favorite game IDs first for efficient checking
	var user models.User
	database.DB.Preload("FavoriteGames").First(&user, userID)
	favoriteIDs := make(map[uint]bool)
	var favGameIDs []uint
	for _, favGame := range user.FavoriteGames {
		favoriteIDs[favGame.ID] = true
		favGameIDs = append(favGameIDs, favGame.ID)
	}

	var totalItems int64
	
	// Create the base query for both counting and data retrieval
	dbQuery := database.DB.Model(&models.Game{})

	// Filter by favorites only
	if favoritesOnly {
		if len(favGameIDs) == 0 { // If no favorites, return empty paginated response
			c.JSON(http.StatusOK, NewPaginatedResponse([]GameResponse{}, 0, page, limit))
			return
		}
		dbQuery = dbQuery.Where("id IN (?)", favGameIDs)
	}

	// Filter by name
	if searchQuery != "" {
		dbQuery = dbQuery.Where("name ILIKE ?", "%"+searchQuery+"%")
	}

	// Filter by tags
    var tagIDs []uint
	if tagIDsStr != "" {
		for _, s := range splitCommaSeparated(tagIDsStr) {
			if id, parseErr := strconv.ParseUint(s, 10, 32); parseErr == nil {
				tagIDs = append(tagIDs, uint(id))
			}
		}
    }

    if len(tagIDs) > 0 {
        dbQuery = dbQuery.Joins("JOIN game_tags gt ON gt.game_id = games.id").
            Where("gt.tag_id IN (?)", tagIDs).
            Group("games.id")
    }

	// --- Count total items ---
    // We need a separate query for counting when using GROUP BY
    // to avoid GORM's default behavior which can be incorrect.
    countQuery := database.DB.Model(&models.Game{})
    if favoritesOnly {
        countQuery = countQuery.Where("id IN (?)", favGameIDs)
    }
    if searchQuery != "" {
        countQuery = countQuery.Where("name ILIKE ?", "%"+searchQuery+"%")
    }
    if len(tagIDs) > 0 {
        // For a grouped query, we count the number of distinct groups.
        // Creating a subquery for the count is a robust way to do this.
        subQuery := countQuery.Joins("JOIN game_tags gt ON gt.game_id = games.id").
            Where("gt.tag_id IN (?)", tagIDs).
            Group("games.id").Select("games.id")
        
        if err := database.DB.Table("(?) as sub", subQuery).Count(&totalItems).Error; err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count games"})
		    return
        }
    } else {
        if err := countQuery.Count(&totalItems).Error; err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count games"})
		    return
        }
    }

	// --- Fetch paginated data ---
	var games []models.Game
	err = dbQuery.Preload("Tags").Offset(offset).Limit(limit).Find(&games).Error
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve games"})
		return
    }

	var response []GameResponse
	for _, game := range games {
		response = append(response, newGameResponse(game, favoriteIDs))
	}

	c.JSON(http.StatusOK, NewPaginatedResponse(response, totalItems, page, limit))
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
