package auth

import (
	"net/http"
	"playmatch/backend/internal/database"
	"playmatch/backend/internal/models"

	"github.com/gin-gonic/gin"
)

// AdminMiddleware creates a gin middleware to check for admin role.
// It must be used AFTER the standard AuthMiddleware.
func AdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("userID")
		if !exists {
			// This should not happen if AuthMiddleware is used before it
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
			return
		}

		var user models.User
		if err := database.DB.First(&user, userID.(uint)).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "Authenticated user not found"})
			return
		}

		if user.Role != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			return
		}

		c.Next()
	}
}
