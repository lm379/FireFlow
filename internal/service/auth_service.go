package service

import (
	"FireFlow/internal/logger"
	"FireFlow/internal/middleware"
	"FireFlow/internal/model"
	"FireFlow/internal/repository"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	DefaultUsername = "admin"
)

type AuthService interface {
	Login(username, password string) (*model.LoginResponse, error)
	ChangePassword(userID uint, oldPassword, newPassword string) error
	VerifyToken(token string) (*model.VerifyTokenResponse, error)
	GetUserByID(id uint) (*model.AuthUser, error)
	InitializeDefaultUser() (string, error)
	IsFirstLogin(userID uint) (bool, error)
	ResetAdminPassword() (string, error)
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

// generateTemporaryPassword 生成由操作系统安全随机源提供的临时密码。
func generateTemporaryPassword() (string, error) {
	var entropy [24]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(entropy[:]), nil
}

// InitializeDefaultUser 创建管理员；已有账户返回空密码。
func (s *authService) InitializeDefaultUser() (string, error) {
	// 检查是否已存在admin用户
	_, err := s.authRepo.GetByUsername(DefaultUsername)
	if err == nil {
		// 用户已存在，不需要创建
		return "", nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		// 其他错误
		return "", err
	}

	// 创建默认用户
	password, err := generateTemporaryPassword()
	if err != nil {
		return "", err
	}
	hashedPassword, err := s.hashPassword(password)
	if err != nil {
		return "", err
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
		return "", err
	}

	logger.InfoLogger.Info("Default admin user created successfully")
	return password, nil
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

	// 令牌版本必须对应本次验证的密码，避免并发重置后旧密码获取新版本令牌。
	token, expiresAt, err := middleware.GenerateToken(user)
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
		User:         user,
		IsFirstLogin: user.IsFirstLogin,
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
	if newPassword == oldPassword {
		return errors.New("new password must differ from old password")
	}

	// 加密新密码
	hashedPassword, err := s.hashPassword(newPassword)
	if err != nil {
		return err
	}

	// 密码、令牌版本和首次登录状态必须同时提交。
	err = s.authRepo.ReplacePassword(userID, user.TokenVersion, hashedPassword, false)
	if err != nil {
		return err
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

// ResetAdminPassword 重置管理员密码并返回新的临时密码。
func (s *authService) ResetAdminPassword() (string, error) {
	// 查找管理员用户
	user, err := s.authRepo.GetByUsername(DefaultUsername)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 如果管理员用户不存在，创建默认用户
			return s.InitializeDefaultUser()
		}
		return "", err
	}

	password, err := generateTemporaryPassword()
	if err != nil {
		return "", err
	}
	hashedPassword, err := s.hashPassword(password)
	if err != nil {
		return "", err
	}

	err = s.authRepo.ReplacePassword(user.ID, user.TokenVersion, hashedPassword, true)
	if err != nil {
		return "", err
	}

	return password, nil
}

// GetUserTokenVersion 获取用户令牌版本
func (s *authService) GetUserTokenVersion(userID uint) (int, error) {
	return s.authRepo.GetUserTokenVersion(userID)
}
