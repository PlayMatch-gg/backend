package handler

import (
	"fmt"
	"net/http"
	"playmatch/backend/internal/database"
	"playmatch/backend/internal/hub"
	"playmatch/backend/internal/models"
	"strconv"
	"time"
	"io" // Import the io package

	"github.com/gin-gonic/gin"
)

// region --- DTOs ---

type LobbyInput struct {
	GameID      uint   `json:"game_id" binding:"required"`
	Description string `json:"description"`
	MaxPlayers  int    `json:"max_players" binding:"required,min=2,max=10"`
}

type LobbyResponse struct {
	ID          uint                 `json:"id"`
	Description string               `json:"description"`
	MaxPlayers  int                  `json:"max_players"`
	Game        GameResponse         `json:"game"`
	Host        PublicUserResponse   `json:"host"`
	Members     []PublicUserResponse `json:"members"`
}

type MessageInput struct {
	Content string `json:"content" binding:"required"`
}

type MessageResponse struct {
	ID        uint                     `json:"id"`
	LobbyID   uint                     `json:"lobby_id"`
	UserID    *uint                    `json:"user_id,omitempty"`
	Type      models.MessageType       `json:"type"`
	Content   string                   `json:"content"`
	CreatedAt time.Time                `json:"created_at"`
	User      *PublicUserResponse      `json:"user,omitempty"`
}

func newLobbyResponse(lobby models.Lobby) LobbyResponse {
	var memberResponses []PublicUserResponse
	for _, member := range lobby.Members {
		// Pass 0 as viewerID since we don't have that context here
		// This part can be enhanced if needed
		memberResponses = append(memberResponses, buildPublicUserResponse(member, 0))
	}

	hostResponse := buildPublicUserResponse(lobby.Host, 0)
	
	// Create a dummy favoriteIDs map for newGameResponse as we don't have user context here
	// This ensures the GameResponse is properly formed without a user's favorite status
	dummyFavoriteIDs := make(map[uint]bool) 
	gameResponse := newGameResponse(lobby.Game, dummyFavoriteIDs)

	return LobbyResponse{
		ID:          lobby.ID,
		Description: lobby.Description,
		MaxPlayers:  lobby.MaxPlayers,
		Game:        gameResponse,
		Host:        hostResponse,
		Members:     memberResponses,
	}
}

func newMessageResponse(message models.Message) MessageResponse {
	var userResponse *PublicUserResponse
	if message.UserID != nil {
		tempUserResponse := buildPublicUserResponse(message.User, 0) // Pass 0 as viewerID
		userResponse = &tempUserResponse
	}
	return MessageResponse{
		ID:        message.ID,
		LobbyID:   message.LobbyID,
		UserID:    message.UserID,
		Type:      message.Type,
		Content:   message.Content,
		CreatedAt: message.CreatedAt,
		User:      userResponse,
	}
}

// endregion

// --- Chat/Event Handlers ---

// SubscribeToLobbyEvents godoc
// @Summary      Subscribe to lobby events (SSE)
// @Description  Establishes a Server-Sent Events connection to receive real-time updates for a lobby.
// @Tags         lobbies-chat
// @Produce      text/event-stream
// @Security     BearerAuth
// @Param        id   path      int  true  "Lobby ID"
// @Success      200 {string} string "Event stream"
// @Failure      401 {object} ErrorResponse
// @Failure      403 {object} ErrorResponse "Not a member of this lobby"
// @Failure      404 {object} ErrorResponse "Lobby not found"
// @Router       /lobbies/{id}/events [get]
func SubscribeToLobbyEvents(c *gin.Context) {
	userID, _ := c.Get("userID")
	lobbyID, _ := strconv.Atoi(c.Param("id"))

	// Check if user is a member of the lobby
	var user models.User
	if err := database.DB.Preload("CurrentLobby").First(&user, userID).Error; err != nil || user.CurrentLobbyID == nil || *user.CurrentLobbyID != uint(lobbyID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not a member of this lobby or lobby not found"})
		return
	}

	clientChan := make(hub.Client)
	hub.GlobalHub.Subscribe(uint(lobbyID), clientChan)

	defer func() {
		hub.GlobalHub.Unsubscribe(uint(lobbyID), clientChan)
	}()

	c.Stream(func(w io.Writer) bool { // Changed from http.ResponseWriter to io.Writer
		select {
		case message := <-clientChan:
			c.SSEvent("message", string(message))
			return true
		case <-c.Request.Context().Done():
			return false
		}
	})
}

// PostMessage godoc
// @Summary      Post a message to lobby chat
// @Description  Sends a new chat message to the specified lobby.
// @Tags         lobbies-chat
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path      int            true  "Lobby ID"
// @Param        input body      MessageInput true  "Message Content"
// @Success      201   {object}  MessageResponse
// @Failure      400   {object}  ErrorResponse
// @Failure      401   {object}  ErrorResponse
// @Failure      403   {object}  ErrorResponse "Not a member of this lobby"
// @Failure      404   {object}  ErrorResponse "Lobby not found"
// @Failure      500   {object}  ErrorResponse
// @Router       /lobbies/{id}/messages [post]
func PostMessage(c *gin.Context) {
	userID, _ := c.Get("userID")
	lobbyID, _ := strconv.Atoi(c.Param("id"))

	// Check if user is a member of the lobby
	var user models.User
	if err := database.DB.Preload("CurrentLobby").First(&user, userID).Error; err != nil || user.CurrentLobbyID == nil || *user.CurrentLobbyID != uint(lobbyID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not a member of this lobby or lobby not found"})
		return
	}

	var input MessageInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	newMessage := models.Message{
		LobbyID: uint(lobbyID),
		UserID:  &user.ID, // User-sent message
		Type:    models.MessageTypeText,
		Content: input.Content,
	}

	if err := database.DB.Create(&newMessage).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to post message"})
		return
	}
	
	// Preload user for the message response
	database.DB.Preload("User").First(&newMessage, newMessage.ID)

	// Broadcast new message event
	hub.GlobalHub.Broadcast(uint(lobbyID), hub.Event{
		Type:    "new_message",
		Payload: newMessageResponse(newMessage),
	})

	c.JSON(http.StatusCreated, newMessageResponse(newMessage))
}

// GetMessages godoc
// @Summary      Get lobby chat messages
// @Description  Retrieves a paginated history of messages for a specific lobby.
// @Tags         lobbies-chat
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "Lobby ID"
// @Param        page query     int  false "Page number" default(1)
// @Param        limit query    int  false "Items per page" default(50)
// @Success      200 {array} MessageResponse
// @Failure      401 {object} ErrorResponse
// @Failure      403 {object} ErrorResponse "Not a member of this lobby"
// @Failure      404 {object} ErrorResponse "Lobby not found"
// @Router       /lobbies/{id}/messages [get]
func GetMessages(c *gin.Context) {
	userID, _ := c.Get("userID")
	lobbyID, _ := strconv.Atoi(c.Param("id"))

	// Check if user is a member of the lobby
	var user models.User
	if err := database.DB.Preload("CurrentLobby").First(&user, userID).Error; err != nil || user.CurrentLobbyID == nil || *user.CurrentLobbyID != uint(lobbyID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not a member of this lobby or lobby not found"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset := (page - 1) * limit

	var messages []models.Message
	database.DB.Where("lobby_id = ?", lobbyID).
		Preload("User").
		Order("created_at DESC"). // Latest messages first
		Limit(limit).Offset(offset).
		Find(&messages)

	// Reverse messages to show oldest first in display
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	var response []MessageResponse
	for _, msg := range messages {
		response = append(response, newMessageResponse(msg))
	}

	c.JSON(http.StatusOK, response)
}

// CreateLobby godoc
// @Summary      Create a new lobby
// @Description  Creates a new lobby, making the creator the host.
// @Tags         lobbies
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        input body LobbyInput true "Lobby Info"
// @Success      201  {object}  LobbyResponse
// @Failure      400  {object}  ErrorResponse
// @Failure      401  {object}  ErrorResponse
// @Failure      409  {object}  ErrorResponse "User is already in a lobby"
// @Router       /lobbies [post]
func CreateLobby(c *gin.Context) {
	userID, _ := c.Get("userID")

	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	if user.CurrentLobbyID != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "User is already in a lobby"})
		return
	}

	var input LobbyInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	lobby := models.Lobby{
		GameID:      input.GameID,
		HostID:      user.ID,
		Description: input.Description,
		MaxPlayers:  input.MaxPlayers,
	}

	// Use a transaction to ensure both lobby creation and user update succeed
	tx := database.DB.Begin()

	if err := tx.Create(&lobby).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create lobby"})
		return
	}

	user.CurrentLobbyID = &lobby.ID
	if err := tx.Save(&user).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user's lobby"})
		return
	}

	tx.Commit()

	// Reload lobby with all associations
	database.DB.Preload("Game").Preload("Host").Preload("Members").First(&lobby, lobby.ID)

	// Post system message
	systemMessage := models.Message{
		LobbyID: lobby.ID,
		UserID:  nil, // System message
		Type:    models.MessageTypeSystem,
		Content: fmt.Sprintf("User %s created the lobby.", user.Nickname),
	}
	database.DB.Create(&systemMessage) // Not in transaction, best-effort

	// Broadcast event
	hub.GlobalHub.Broadcast(lobby.ID, hub.Event{
		Type:    "lobby_created",
		Payload: newLobbyResponse(lobby),
	})

	c.JSON(http.StatusCreated, newLobbyResponse(lobby))
}

// SearchLobbies godoc
// @Summary      Search for lobbies
// @Description  Gets a paginated list of available lobbies, optionally filtered by game.
// @Tags         lobbies
// @Produce      json
// @Security     BearerAuth
// @Param        game_id query int false "Filter by Game ID"
// @Param        page    query int false "Page number" default(1)
// @Param        limit   query int false "Items per page" default(10)
// @Success      200 {array} LobbyResponse
// @Router       /lobbies [get]
func SearchLobbies(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset := (page - 1) * limit
	gameID := c.Query("game_id")

	var lobbies []models.Lobby
	query := database.DB.Model(&models.Lobby{}).
		Preload("Game").
		Preload("Host").
		Preload("Members").
		Joins("LEFT JOIN users ON users.current_lobby_id = lobbies.id").
		Group("lobbies.id").
		Having("COUNT(users.id) < lobbies.max_players") // Filter out full lobbies

	if gameID != "" {
		query = query.Where("lobbies.game_id = ?", gameID)
	}

	query.Offset(offset).Limit(limit).Find(&lobbies)

	var response []LobbyResponse
	for _, lobby := range lobbies {
		response = append(response, newLobbyResponse(lobby))
	}

	c.JSON(http.StatusOK, response)
}

// GetLobbyByID godoc
// @Summary      Get a lobby by ID
// @Description  Gets full details for a single lobby.
// @Tags         lobbies
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "Lobby ID"
// @Success      200 {object} LobbyResponse
// @Failure      404 {object} ErrorResponse "Lobby not found"
// @Router       /lobbies/{id} [get]
func GetLobbyByID(c *gin.Context) {
	lobbyID, _ := strconv.Atoi(c.Param("id"))

	var lobby models.Lobby
	if err := database.DB.Preload("Game").Preload("Host").Preload("Members").First(&lobby, lobbyID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Lobby not found"})
		return
	}

	c.JSON(http.StatusOK, newLobbyResponse(lobby))
}

// JoinLobby godoc
// @Summary      Join a lobby
// @Description  Joins a lobby if not full and the user is not already in another lobby.
// @Tags         lobbies
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "Lobby ID"
// @Success      200 {object} map[string]string "{"message": "Joined lobby successfully"}"
// @Failure      404 {object} ErrorResponse "Lobby not found"
// @Failure      409 {object} ErrorResponse "Lobby is full or user is in another lobby"
// @Router       /lobbies/{id}/join [post]
func JoinLobby(c *gin.Context) {
	userID, _ := c.Get("userID")
	lobbyID, _ := strconv.Atoi(c.Param("id"))

	// Check user isn't already in a lobby
	var user models.User
	database.DB.First(&user, userID)
	if user.CurrentLobbyID != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "User is already in a lobby"})
		return
	}

	// Check lobby exists and is not full
	var lobby models.Lobby
	if err := database.DB.Preload("Members").First(&lobby, lobbyID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Lobby not found"})
		return
	}
	if len(lobby.Members) >= lobby.MaxPlayers {
		c.JSON(http.StatusConflict, gin.H{"error": "Lobby is full"})
		return
	}

	// Join lobby
	lobbyIDUint := uint(lobbyID)
	database.DB.Model(&user).Update("current_lobby_id", &lobbyIDUint)

	// Post system message
	systemMessage := models.Message{
		LobbyID: lobby.ID,
		UserID:  nil, // System message
		Type:    models.MessageTypeSystem,
		Content: fmt.Sprintf("User %s joined the lobby.", user.Nickname),
	}
	database.DB.Create(&systemMessage)

	// Broadcast event
	hub.GlobalHub.Broadcast(lobby.ID, hub.Event{
		Type:    "user_joined",
		Payload: buildPublicUserResponse(user, 0), // User who joined
	})

	c.JSON(http.StatusOK, gin.H{"message": "Joined lobby successfully"})
}

// LeaveLobby godoc
// @Summary      Leave the current lobby
// @Description  Leaves the lobby the user is currently in. Handles host migration and lobby deletion.
// @Tags         lobbies
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} map[string]string "{"message": "Left lobby successfully"}"
// @Failure      404 {object} ErrorResponse "User is not in a lobby"
// @Router       /lobbies/leave [post]
func LeaveLobby(c *gin.Context) {
	userID, _ := c.Get("userID")

	var user models.User
	if err := database.DB.Preload("CurrentLobby.Members").First(&user, userID).Error; err != nil || user.CurrentLobbyID == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User is not in a lobby"})
		return
	}

	lobby := user.CurrentLobby

	// Use transaction for leaving logic
	tx := database.DB.Begin()

	// User leaves the lobby
	if err := tx.Model(&user).Update("current_lobby_id", nil).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to leave lobby"})
		return
	}

	// Post system message for user leaving
	systemMessage := models.Message{
		LobbyID: lobby.ID,
		UserID:  nil, // System message
		Type:    models.MessageTypeSystem,
		Content: fmt.Sprintf("User %s left the lobby.", user.Nickname),
	}
	tx.Create(&systemMessage) // Part of transaction

	// If the user was the last one, delete the lobby
	if len(lobby.Members) == 1 && lobby.Members[0].ID == user.ID {
		if err := tx.Delete(&lobby).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete empty lobby"})
			return
		}
		// Broadcast lobby deleted event
		hub.GlobalHub.Broadcast(lobby.ID, hub.Event{
			Type:    "lobby_deleted",
			Payload: gin.H{"lobby_id": lobby.ID},
		})
	} else if lobby.HostID == user.ID { // If the user was the host, promote the next member
		var nextHost models.User
		for _, member := range lobby.Members {
			if member.ID != user.ID {
				nextHost = member
				break
			}
		}
		if nextHost.ID != 0 {
			if err := tx.Model(&lobby).Update("host_id", nextHost.ID).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to transfer host"})
				return
			}
			// Post system message for new host
			newHostMessage := models.Message{
				LobbyID: lobby.ID,
				UserID:  nil, // System message
				Type:    models.MessageTypeSystem,
				Content: fmt.Sprintf("User %s is now the host.", nextHost.Nickname),
			}
			tx.Create(&newHostMessage)

			// Broadcast new host event
			hub.GlobalHub.Broadcast(lobby.ID, hub.Event{
				Type:    "host_changed",
				Payload: buildPublicUserResponse(nextHost, 0),
			})
		}
	}

	tx.Commit()

	// Broadcast user left event
	hub.GlobalHub.Broadcast(lobby.ID, hub.Event{
		Type:    "user_left",
		Payload: buildPublicUserResponse(user, 0),
	})

	c.JSON(http.StatusOK, gin.H{"message": "Left lobby successfully"})
}

// UpdateLobby godoc
// @Summary      Update a lobby (Host only)
// @Description  Updates the details of a lobby. Only the host can perform this action.
// @Tags         lobbies
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path      int       true  "Lobby ID"
// @Param        input body      LobbyInput true  "New Lobby Info"
// @Success      200   {object}  LobbyResponse
// @Failure      403   {object}  ErrorResponse "Only the host can update the lobby"
// @Failure      404   {object}  ErrorResponse "Lobby not found"
// @Router       /lobbies/{id} [put]
func UpdateLobby(c *gin.Context) {
	userID, _ := c.Get("userID")
	lobbyID, _ := strconv.Atoi(c.Param("id"))

	var lobby models.Lobby
	if err := database.DB.First(&lobby, lobbyID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Lobby not found"})
		return
	}

	if lobby.HostID != userID.(uint) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only the host can update the lobby"})
		return
	}

	var input LobbyInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get old game for comparison
	var oldGame models.Game
	database.DB.First(&oldGame, lobby.GameID)

	lobby.Description = input.Description
	lobby.MaxPlayers = input.MaxPlayers
	lobby.GameID = input.GameID

	database.DB.Save(&lobby)

	database.DB.Preload("Game").Preload("Host").Preload("Members").First(&lobby, lobby.ID)

	// Post system message if game changed
	if oldGame.ID != lobby.Game.ID {
		systemMessage := models.Message{
			LobbyID: lobby.ID,
			UserID:  nil, // System message
			Type:    models.MessageTypeSystem,
			Content: fmt.Sprintf("Lobby game changed to %s.", lobby.Game.Name),
		}
		database.DB.Create(&systemMessage)
		// Broadcast event
		hub.GlobalHub.Broadcast(lobby.ID, hub.Event{
			Type:    "lobby_game_changed",
			Payload: newGameResponse(lobby.Game, nil), // Updated game info
		})
	}
	// Broadcast general lobby update
	hub.GlobalHub.Broadcast(lobby.ID, hub.Event{
		Type:    "lobby_updated",
		Payload: newLobbyResponse(lobby),
	})

	c.JSON(http.StatusOK, newLobbyResponse(lobby))
}

// KickMember godoc
// @Summary      Kick a member from a lobby (Host only)
// @Description  Removes a member from the lobby. Only the host can perform this action.
// @Tags         lobbies
// @Produce      json
// @Security     BearerAuth
// @Param        id      path int true "Lobby ID"
// @Param        userID  path int true "User ID of member to kick"
// @Success      200 {object} map[string]string "{"message": "Member kicked successfully"}"
// @Failure      403 {object} ErrorResponse "Only the host can kick members"
// @Failure      404 {object} ErrorResponse "Lobby or member not found"
// @Router       /lobbies/{id}/members/{userID} [delete]
func KickMember(c *gin.Context) {
	hostID, _ := c.Get("userID")
	lobbyID, _ := strconv.Atoi(c.Param("id"))
	memberToKickID, _ := strconv.Atoi(c.Param("userID"))

	var lobby models.Lobby
	if err := database.DB.First(&lobby, lobbyID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Lobby not found"})
		return
	}

	if lobby.HostID != hostID.(uint) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only the host can kick members"})
		return
	}
	
	if lobby.HostID == uint(memberToKickID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Host cannot kick themselves"})
		return
	}

	var memberToKick models.User
	if err := database.DB.Where("id = ? AND current_lobby_id = ?", memberToKickID, lobbyID).First(&memberToKick).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Member not found in this lobby"})
		return
	}

	database.DB.Model(&memberToKick).Update("current_lobby_id", nil)
	
	// Post system message
	systemMessage := models.Message{
		LobbyID: lobby.ID,
		UserID:  nil, // System message
		Type:    models.MessageTypeSystem,
		Content: fmt.Sprintf("User %s was kicked from the lobby.", memberToKick.Nickname),
	}
	database.DB.Create(&systemMessage)

	// Broadcast user kicked event
	hub.GlobalHub.Broadcast(lobby.ID, hub.Event{
		Type:    "user_kicked",
		Payload: buildPublicUserResponse(memberToKick, 0),
	})

	c.JSON(http.StatusOK, gin.H{"message": "Member kicked successfully"})
}
