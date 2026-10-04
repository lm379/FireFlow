package middleware

import (
	"FireFlow/internal/response"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func IsAPIRequest(c *gin.Context) bool {
	return c.Request.URL.Path == "/api" || strings.HasPrefix(c.Request.URL.Path, "/api/")
}

func APIRecovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		if IsAPIRequest(c) {
			c.AbortWithStatusJSON(http.StatusInternalServerError, response.Error("Internal server error"))
			return
		}
		c.AbortWithStatus(http.StatusInternalServerError)
	})
}
