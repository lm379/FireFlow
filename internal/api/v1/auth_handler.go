package v1

import (
	"FireFlow/internal/dto"
	"FireFlow/internal/logger"
	"FireFlow/internal/middleware"
	"FireFlow/internal/model"
	"FireFlow/internal/response"
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
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Router /api/v1/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req model.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.InfoLogger.Warnf("Invalid login request: %v", err)
		c.JSON(http.StatusBadRequest, response.ErrorCode(http.StatusBadRequest, "Invalid request parameters"))
		return
	}

	// 参数验证
	if req.Username == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, response.ErrorCode(http.StatusBadRequest, "Username and password are required"))
		return
	}

	result, err := h.authService.Login(req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.ErrorCode(http.StatusUnauthorized, err.Error()))
		return
	}

	c.JSON(http.StatusOK, response.Success(dto.Login(result), "Login successful"))
}

// ChangePassword 修改密码
// @Summary 修改密码
// @Description 修改当前用户密码
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body model.ChangePasswordRequest true "密码修改信息"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Router /api/v1/auth/change-password [post]
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var req model.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorCode(http.StatusBadRequest, "Invalid request parameters"))
		return
	}

	// 从JWT中获取用户ID
	userID, exists := middleware.GetCurrentUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, response.ErrorCode(http.StatusUnauthorized, "User not authenticated"))
		return
	}

	err := h.authService.ChangePassword(userID, req.OldPassword, req.NewPassword)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorCode(http.StatusBadRequest, err.Error()))
		return
	}

	c.JSON(http.StatusOK, response.Success(nil, "Password changed successfully"))
}

// VerifyToken 验证令牌
// @Summary 验证JWT令牌
// @Description 验证JWT令牌的有效性
// @Tags auth
// @Accept json
// @Produce json
// @Param token query string true "JWT令牌"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
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
		c.JSON(http.StatusBadRequest, response.ErrorCode(http.StatusBadRequest, "Token is required"))
		return
	}

	result, err := h.authService.VerifyToken(token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCode(http.StatusInternalServerError, "Failed to verify token"))
		return
	}

	c.JSON(http.StatusOK, response.Success(dto.VerifyToken(result), "success"))
}

// GetCurrentUser 获取当前用户信息
// @Summary 获取当前用户信息
// @Description 获取当前认证用户的详细信息
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Response
// @Failure 401 {object} response.Response
// @Router /api/v1/auth/me [get]
func (h *AuthHandler) GetCurrentUser(c *gin.Context) {
	userID, exists := middleware.GetCurrentUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, response.ErrorCode(http.StatusUnauthorized, "User not authenticated"))
		return
	}

	user, err := h.authService.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCode(http.StatusInternalServerError, "Failed to get user information"))
		return
	}

	c.JSON(http.StatusOK, response.Success(dto.User(user), "success"))
}

// CheckFirstLogin 检查是否为首次登录
// @Summary 检查首次登录状态
// @Description 检查当前用户是否为首次登录
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Response
// @Failure 401 {object} response.Response
// @Router /api/v1/auth/first-login [get]
func (h *AuthHandler) CheckFirstLogin(c *gin.Context) {
	userID, exists := middleware.GetCurrentUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, response.ErrorCode(http.StatusUnauthorized, "User not authenticated"))
		return
	}

	isFirstLogin, err := h.authService.IsFirstLogin(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCode(http.StatusInternalServerError, "Failed to check first login status"))
		return
	}

	c.JSON(http.StatusOK, response.Success(gin.H{"is_first_login": isFirstLogin}, "success"))
}

// Logout 用户退出登录
// @Summary 用户退出登录
// @Description 用户退出登录（客户端清除令牌）
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Response
// @Router /api/v1/auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	// 在JWT无状态认证中，登出主要由客户端处理（删除令牌）
	// 服务端可以记录登出日志
	username, _ := middleware.GetCurrentUsername(c)
	if username != "" {
		logger.InfoLogger.Infof("User %s logged out", username)
	}

	c.JSON(http.StatusOK, response.Success(nil, "Logout successful"))
}

// RefreshToken 刷新令牌
// @Summary 刷新JWT令牌
// @Description 使用当前有效令牌刷新获取新令牌
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Response
// @Failure 401 {object} response.Response
// @Router /api/v1/auth/refresh [post]
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	userID, exists := middleware.GetCurrentUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, response.ErrorCode(http.StatusUnauthorized, "User not authenticated"))
		return
	}

	user, err := h.authService.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCode(http.StatusInternalServerError, "Failed to get user information"))
		return
	}

	// 生成新的令牌
	token, expiresAt, err := middleware.GenerateToken(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCode(http.StatusInternalServerError, "Failed to generate new token"))
		return
	}

	result := &model.LoginResponse{
		Token:        token,
		User:         user,
		IsFirstLogin: user.IsFirstLogin,
		ExpiresAt:    expiresAt,
	}

	c.JSON(http.StatusOK, response.Success(dto.Login(result), "Token refreshed successfully"))
}
