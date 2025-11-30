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

		        		// User routes
		        		userRoutes := apiV1.Group("/users")
		        		userRoutes.Use(auth.OptionalAuthMiddleware()) // Use optional auth for public user data
		        		{
		        			userRoutes.GET("", handler.SearchUsers) // Must be before /:id
		        			userRoutes.GET("/:id", handler.GetUserByID)
		        
		        			// Protected user routes
		        			protectedUserRoutes := userRoutes.Group("")
		        			protectedUserRoutes.Use(auth.AuthMiddleware())
		        			{
		        				protectedUserRoutes.GET("/me", handler.GetMe)
		        				protectedUserRoutes.GET("/me/relations", handler.GetRelations)
		        				protectedUserRoutes.GET("/:id/relations", handler.GetUserRelationsByID)
		        
		        				// Friendship routes
		        				protectedUserRoutes.POST("/:id/request", handler.SendRequest)
		        				protectedUserRoutes.POST("/:id/accept", handler.AcceptRequest)
		        				protectedUserRoutes.POST("/:id/decline", handler.DeclineRequest)
		        				protectedUserRoutes.POST("/:id/remove", handler.RemoveRelation)
		        			}
		        		}
		        
		        		// Game routes
		        		gameRoutes := apiV1.Group("/games")
		        		gameRoutes.Use(auth.OptionalAuthMiddleware()) // Use optional auth for public game data
		        		{
		        			gameRoutes.GET("", handler.GetGames)
		        			gameRoutes.GET("/:id", handler.GetGameByID)
		        
		        			// Protected game routes
		        			protectedGameRoutes := gameRoutes.Group("")
		        			protectedGameRoutes.Use(auth.AuthMiddleware())
		        			{
		        				protectedGameRoutes.POST("/:id/favorite", handler.ToggleFavoriteGame)
		        			}
		        		}
		        
		        				// Lobby routes
		        
		        				lobbyRoutes := apiV1.Group("/lobbies")
		        
		        				lobbyRoutes.Use(auth.OptionalAuthMiddleware()) // Use optional auth for public lobby data
		        
		        				{
		        
		        					lobbyRoutes.GET("", handler.SearchLobbies)
		        
		        					lobbyRoutes.GET("/:id", handler.GetLobbyByID)
		        
		        					
		        
		        					// Protected lobby routes for the current user
		        
		        					meLobbyRoutes := lobbyRoutes.Group("/me")
		        
		        					meLobbyRoutes.Use(auth.AuthMiddleware())
		        
		        					{
		        
		        						meLobbyRoutes.GET("", handler.GetMyLobby)
		        
		        						meLobbyRoutes.PUT("", handler.UpdateLobby)
		        
		        						meLobbyRoutes.POST("/leave", handler.LeaveLobby)
		        
		        						meLobbyRoutes.DELETE("/members/:userID", handler.KickMember)
		        
		        		
		        
		        						// Chat and Events
		        
		        						meLobbyRoutes.GET("/events", handler.SubscribeToLobbyEvents)
		        
		        						meLobbyRoutes.POST("/messages", handler.PostMessage)
		        
		        						meLobbyRoutes.GET("/messages", handler.GetMessages)

										meLobbyRoutes.POST("/typing", handler.PostUserTyping)
		        
		        					}
		        
		        		
		        
		        					// Other protected lobby routes
		        
		        					protectedLobbyRoutes := lobbyRoutes.Group("")
		        
		        					protectedLobbyRoutes.Use(auth.AuthMiddleware())
		        
		        					{
		        
		        						protectedLobbyRoutes.POST("", handler.CreateLobby)
		        
		        						protectedLobbyRoutes.POST("/:id/join", handler.JoinLobby)
		        
		        					}
		        
		        				}		// Admin routes (protected by auth and admin check)
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
