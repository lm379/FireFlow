package service

import (
	"FireFlow/internal/logger"
	"FireFlow/internal/middleware"
	"FireFlow/internal/model"
	"FireFlow/internal/repository"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	DefaultUsername = "admin"
	DefaultPassword = "password"
)

type AuthService interface {
	Login(username, password string) (*model.LoginResponse, error)
	ChangePassword(userID uint, oldPassword, newPassword string) error
	VerifyToken(token string) (*model.VerifyTokenResponse, error)
	GetUserByID(id uint) (*model.AuthUser, error)
	InitializeDefaultUser() error
	IsFirstLogin(userID uint) (bool, error)
	ResetAdminPassword() error
	GetUserTokenVersion(userID uint) (int, error)
}

type authService struct {
	authRepo repository.AuthUserRepository
}

func NewAuthService(authRepo repository.AuthUserRepository) AuthService {
	return &authService{
		authRepo: authRepo,
	}
}

// hashPassword 加密密码
func (s *authService) hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// verifyPassword 验证密码
func (s *authService) verifyPassword(hashedPassword, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil
}

// InitializeDefaultUser 初始化默认用户
func (s *authService) InitializeDefaultUser() error {
	// 检查是否已存在admin用户
	_, err := s.authRepo.GetByUsername(DefaultUsername)
	if err == nil {
		// 用户已存在，不需要创建
		return nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		// 其他错误
		return err
	}

	// 创建默认用户
	hashedPassword, err := s.hashPassword(DefaultPassword)
	if err != nil {
		return err
	}

	defaultUser := &model.AuthUser{
		Username:     DefaultUsername,
		Password:     hashedPassword,
		IsFirstLogin: true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	err = s.authRepo.Create(defaultUser)
	if err != nil {
		return err
	}

	logger.InfoLogger.Info("Default admin user created successfully")
	return nil
}

// Login 用户登录
func (s *authService) Login(username, password string) (*model.LoginResponse, error) {
	// 查找用户
	user, err := s.authRepo.GetByUsername(username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.InfoLogger.Warnf("Login attempt with non-existent username: %s", username)
			return nil, errors.New("invalid username or password")
		}
		return nil, err
	}

	// 验证密码
	if !s.verifyPassword(user.Password, password) {
		logger.InfoLogger.Warnf("Login attempt with invalid password for username: %s", username)
		return nil, errors.New("invalid username or password")
	}

	// 重新获取用户信息以确保令牌版本是最新的（防止密码修改后令牌版本不同步）
	latestUser, err := s.authRepo.GetByID(user.ID)
	if err != nil {
		logger.ErrorLogger.Errorf("Failed to get latest user info for %s: %v", username, err)
		return nil, errors.New("failed to retrieve user information")
	}

	// 生成JWT令牌（使用最新的用户信息）
	token, expiresAt, err := middleware.GenerateToken(latestUser)
	if err != nil {
		logger.ErrorLogger.Errorf("Failed to generate token for user %s: %v", username, err)
		return nil, errors.New("failed to generate authentication token")
	}

	// 更新最后登录时间
	err = s.authRepo.UpdateLoginTime(user.ID)
	if err != nil {
		logger.ErrorLogger.Warnf("Failed to update login time for user %s: %v", username, err)
	}

	logger.InfoLogger.Infof("User %s logged in successfully", username)

	return &model.LoginResponse{
		Token:        token,
		User:         latestUser,
		IsFirstLogin: latestUser.IsFirstLogin,
		ExpiresAt:    expiresAt,
	}, nil
}

// ChangePassword 修改密码
func (s *authService) ChangePassword(userID uint, oldPassword, newPassword string) error {
	// 获取用户信息
	user, err := s.authRepo.GetByID(userID)
	if err != nil {
		return err
	}

	// 验证旧密码
	if !s.verifyPassword(user.Password, oldPassword) {
		return errors.New("old password is incorrect")
	}

	// 检查新密码长度
	if len(newPassword) < 6 {
		return errors.New("new password must be at least 6 characters long")
	}

	// 加密新密码
	hashedPassword, err := s.hashPassword(newPassword)
	if err != nil {
		return err
	}

	// 更新密码
	err = s.authRepo.UpdatePassword(userID, hashedPassword)
	if err != nil {
		return err
	}

	// 增加令牌版本，使所有现有令牌失效
	err = s.authRepo.IncrementTokenVersion(userID)
	if err != nil {
		logger.ErrorLogger.Warnf("Failed to increment token version for user %d: %v", userID, err)
	}

	// 如果是首次登录，标记为已完成首次登录
	if user.IsFirstLogin {
		err = s.authRepo.SetFirstLoginCompleted(userID)
		if err != nil {
			logger.ErrorLogger.Warnf("Failed to set first login completed for user %d: %v", userID, err)
		}
	}

	logger.InfoLogger.Infof("Password changed successfully for user %s", user.Username)
	return nil
}

// VerifyToken 验证JWT令牌
func (s *authService) VerifyToken(token string) (*model.VerifyTokenResponse, error) {
	claims, err := middleware.ValidateToken(token)
	if err != nil {
		return &model.VerifyTokenResponse{
			Valid: false,
		}, nil
	}

	// 获取用户信息
	user, err := s.authRepo.GetByID(claims.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &model.VerifyTokenResponse{
				Valid: false,
			}, nil
		}
		return nil, err
	}

	return &model.VerifyTokenResponse{
		Valid:     true,
		User:      user,
		ExpiresAt: claims.ExpiresAt.Unix(),
	}, nil
}

// GetUserByID 根据ID获取用户信息
func (s *authService) GetUserByID(id uint) (*model.AuthUser, error) {
	return s.authRepo.GetByID(id)
}

// IsFirstLogin 检查是否为首次登录
func (s *authService) IsFirstLogin(userID uint) (bool, error) {
	user, err := s.authRepo.GetByID(userID)
	if err != nil {
		return false, err
	}
	return user.IsFirstLogin, nil
}

// ResetAdminPassword 重置管理员密码为默认密码
func (s *authService) ResetAdminPassword() error {
	// 查找管理员用户
	user, err := s.authRepo.GetByUsername(DefaultUsername)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 如果管理员用户不存在，创建默认用户
			return s.InitializeDefaultUser()
		}
		return err
	}

	// 加密默认密码
	hashedPassword, err := s.hashPassword(DefaultPassword)
	if err != nil {
		return err
	}

	// 更新密码并设置首次登录标志
	user.Password = hashedPassword
	user.IsFirstLogin = true
	user.UpdatedAt = time.Now()

	err = s.authRepo.Update(user)
	if err != nil {
		return err
	}

	// 增加令牌版本，使所有现有令牌失效
	err = s.authRepo.IncrementTokenVersion(user.ID)
	if err != nil {
		logger.ErrorLogger.Warnf("Failed to increment token version for user %d: %v", user.ID, err)
	}

	return nil
}

// GetUserTokenVersion 获取用户令牌版本
func (s *authService) GetUserTokenVersion(userID uint) (int, error) {
	return s.authRepo.GetUserTokenVersion(userID)
}
