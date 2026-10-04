package middleware

import (
	"FireFlow/internal/response"
	"net/http"

	"github.com/gin-gonic/gin"
)

type FirstLoginChecker interface {
	IsFirstLogin(userID uint) (bool, error)
}

// RequirePasswordChangeCompleted 限制临时密码登录后的业务权限。
func RequirePasswordChangeCompleted(checker FirstLoginChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := GetCurrentUserID(c)
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, response.Unauthorized("User not authenticated"))
			return
		}
		firstLogin, err := checker.IsFirstLogin(userID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, response.Error("Failed to check password change status"))
			return
		}
		if firstLogin {
			allowed := false
			switch c.FullPath() {
			case "/api/v1/auth/change-password", "/api/v1/auth/logout":
				allowed = c.Request.Method == http.MethodPost
			case "/api/v1/auth/me", "/api/v1/auth/first-login":
				allowed = c.Request.Method == http.MethodGet
			}
			if !allowed {
				c.AbortWithStatusJSON(http.StatusForbidden, response.New(403, nil, "请先修改临时密码", "PASSWORD_CHANGE_REQUIRED"))
				return
			}
		}
		c.Next()
	}
}
