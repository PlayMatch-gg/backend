package main

import (
	"fmt"
	"log"
	"net/http"

	"playmatch/backend/internal/auth"
	"playmatch/backend/internal/config"
	"playmatch/backend/internal/database"
	"playmatch/backend/internal/handler"

	"github.com/gin-gonic/gin"

	// Swagger imports
	_ "playmatch/backend/docs" // This is important for swag to find the generated docs

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func init() {
	config.LoadConfig()
}

// @title           Playmatch API
// @version         1.0
// @description     This is the API for the Playmatch service.
// @host            localhost:8080
// @BasePath        /api/v1
// @securityDefinitions.apiKey BearerAuth
// @in header
// @name Authorization
func main() {
	// Connect to the database
	database.Connect(config.AppConfig.DatabaseURL)

	router := gin.Default()

	// Swagger route
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Health check endpoint
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})

	// API v1 routes
	apiV1 := router.Group("/api/v1")
	{
		// Auth routes
		authRoutes := apiV1.Group("/auth")
		{
			authRoutes.POST("/register", handler.RegisterUser)
			authRoutes.POST("/login", handler.LoginUser)
		}

		// User routes (protected)
		userRoutes := apiV1.Group("/users")
		userRoutes.Use(auth.AuthMiddleware())
		{
			userRoutes.GET("", handler.SearchUsers) // Must be before /:id
			userRoutes.GET("/me", handler.GetMe)
			userRoutes.GET("/me/relations", handler.GetRelations)
			userRoutes.GET("/:id", handler.GetUserByID)
			userRoutes.GET("/:id/relations", handler.GetUserRelationsByID)

			// Friendship routes
			userRoutes.POST("/:id/request", handler.SendRequest)
			userRoutes.POST("/:id/accept", handler.AcceptRequest)
			userRoutes.POST("/:id/decline", handler.DeclineRequest)
			userRoutes.POST("/:id/remove", handler.RemoveRelation)
		}

		// Public Game routes (protected)
		gameRoutes := apiV1.Group("/games")
		gameRoutes.Use(auth.AuthMiddleware())
		{
			gameRoutes.GET("", handler.GetGames)
			gameRoutes.GET("/:id", handler.GetGameByID)
			gameRoutes.POST("/:id/favorite", handler.ToggleFavoriteGame)
		}

		// Lobby routes (protected)
		lobbyRoutes := apiV1.Group("/lobbies")
		lobbyRoutes.Use(auth.AuthMiddleware())
		{
			lobbyRoutes.POST("", handler.CreateLobby)
			lobbyRoutes.GET("", handler.SearchLobbies)
			lobbyRoutes.GET("/:id", handler.GetLobbyByID)
			lobbyRoutes.POST("/:id/join", handler.JoinLobby)
			lobbyRoutes.POST("/leave", handler.LeaveLobby) // No ID needed, user leaves their own lobby
			lobbyRoutes.PUT("/:id", handler.UpdateLobby)
			lobbyRoutes.DELETE("/:id/members/:userID", handler.KickMember)
		}

		// Admin routes (protected by auth and admin check)
		adminRoutes := apiV1.Group("/admin")
		adminRoutes.Use(auth.AuthMiddleware(), auth.AdminMiddleware())
		{
			// Tags CRUD
			tags := adminRoutes.Group("/tags")
			{
				tags.POST("", handler.CreateTag)
				tags.GET("", handler.GetTags)
				tags.PUT("/:id", handler.UpdateTag)
				tags.DELETE("/:id", handler.DeleteTag)
			}

			// Games CRUD (admin-only parts)
			adminGameRoutes := adminRoutes.Group("/games")
			{
				adminGameRoutes.POST("", handler.CreateGame)
				adminGameRoutes.PUT("/:id", handler.UpdateGame)
				adminGameRoutes.DELETE("/:id", handler.DeleteGame)
			}
		}
	}

	fmt.Println("Server is running on :8080")
	fmt.Println("Swagger UI is available at http://localhost:8080/swagger/index.html")
	log.Fatal(router.Run(":8080"))
}
