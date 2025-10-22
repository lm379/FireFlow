package v1

import (
	"FireFlow/internal/logger"
	"FireFlow/internal/middleware"
	"FireFlow/internal/model"
	"FireFlow/internal/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authService service.AuthService
}

func NewAuthHandler(authService service.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}

// Login 用户登录
// @Summary 用户登录
// @Description 用户登录获取JWT令牌
// @Tags auth
// @Accept json
// @Produce json
// @Param body body model.LoginRequest true "登录信息"
// @Success 200 {object} model.LoginResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /api/v1/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req model.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.InfoLogger.Warnf("Invalid login request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request parameters",
			"error":   err.Error(),
		})
		return
	}

	// 参数验证
	if req.Username == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Username and password are required",
		})
		return
	}

	response, err := h.authService.Login(req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Login successful",
		"data":    response,
	})
}

// ChangePassword 修改密码
// @Summary 修改密码
// @Description 修改当前用户密码
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body model.ChangePasswordRequest true "密码修改信息"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /api/v1/auth/change-password [post]
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var req model.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request parameters",
			"error":   err.Error(),
		})
		return
	}

	// 从JWT中获取用户ID
	userID, exists := middleware.GetCurrentUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "User not authenticated",
		})
		return
	}

	err := h.authService.ChangePassword(userID, req.OldPassword, req.NewPassword)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Password changed successfully",
	})
}

// VerifyToken 验证令牌
// @Summary 验证JWT令牌
// @Description 验证JWT令牌的有效性
// @Tags auth
// @Accept json
// @Produce json
// @Param token query string true "JWT令牌"
// @Success 200 {object} model.VerifyTokenResponse
// @Failure 400 {object} map[string]interface{}
// @Router /api/v1/auth/verify [get]
func (h *AuthHandler) VerifyToken(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		// 尝试从Authorization头获取
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" && len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			token = authHeader[7:]
		}
	}

	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Token is required",
		})
		return
	}

	response, err := h.authService.VerifyToken(token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to verify token",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    response,
	})
}

// GetCurrentUser 获取当前用户信息
// @Summary 获取当前用户信息
// @Description 获取当前认证用户的详细信息
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} model.AuthUser
// @Failure 401 {object} map[string]interface{}
// @Router /api/v1/auth/me [get]
func (h *AuthHandler) GetCurrentUser(c *gin.Context) {
	userID, exists := middleware.GetCurrentUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "User not authenticated",
		})
		return
	}

	user, err := h.authService.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to get user information",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    user,
	})
}

// CheckFirstLogin 检查是否为首次登录
// @Summary 检查首次登录状态
// @Description 检查当前用户是否为首次登录
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /api/v1/auth/first-login [get]
func (h *AuthHandler) CheckFirstLogin(c *gin.Context) {
	userID, exists := middleware.GetCurrentUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "User not authenticated",
		})
		return
	}

	isFirstLogin, err := h.authService.IsFirstLogin(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to check first login status",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"is_first_login": isFirstLogin,
	})
}

// Logout 用户退出登录
// @Summary 用户退出登录
// @Description 用户退出登录（客户端清除令牌）
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	// 在JWT无状态认证中，登出主要由客户端处理（删除令牌）
	// 服务端可以记录登出日志
	username, _ := middleware.GetCurrentUsername(c)
	if username != "" {
		logger.InfoLogger.Infof("User %s logged out", username)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Logout successful",
	})
}

// RefreshToken 刷新令牌
// @Summary 刷新JWT令牌
// @Description 使用当前有效令牌刷新获取新令牌
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} model.LoginResponse
// @Failure 401 {object} map[string]interface{}
// @Router /api/v1/auth/refresh [post]
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	userID, exists := middleware.GetCurrentUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "User not authenticated",
		})
		return
	}

	user, err := h.authService.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to get user information",
		})
		return
	}

	// 生成新的令牌
	token, expiresAt, err := middleware.GenerateToken(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to generate new token",
		})
		return
	}

	response := &model.LoginResponse{
		Token:        token,
		User:         user,
		IsFirstLogin: user.IsFirstLogin,
		ExpiresAt:    expiresAt,
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Token refreshed successfully",
		"data":    response,
	})
}
