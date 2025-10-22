package model

import (
	"gorm.io/gorm"
	"time"
)

// AuthUser 用户认证模型
type AuthUser struct {
	ID                uint      `gorm:"primarykey" json:"id"`
	Username          string    `gorm:"column:username;uniqueIndex;not null" json:"username"`
	Password          string    `gorm:"column:password;not null" json:"-"` // 不在JSON中暴露密码
	TokenVersion      int       `gorm:"column:token_version;default:1" json:"-"` // 令牌版本号，用于使令牌失效
	IsFirstLogin      bool      `gorm:"column:is_first_login;default:true" json:"is_first_login"`
	LastLoginTime     *time.Time `gorm:"column:last_login_time" json:"last_login_time"`
	PasswordUpdatedAt *time.Time `gorm:"column:password_updated_at" json:"password_updated_at"`
	CreatedAt         time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt         time.Time `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName 设置表名
func (AuthUser) TableName() string {
	return "auth_users"
}

// AuthToken JWT令牌的Claims结构
type AuthToken struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	IssuedAt int64  `json:"iat"`
	ExpiresAt int64 `json:"exp"`
}

// LoginRequest 登录请求结构
type LoginRequest struct {
	Username string `json:"username" binding:"required" validate:"required"`
	Password string `json:"password" binding:"required" validate:"required"`
}

// ChangePasswordRequest 修改密码请求结构
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required" validate:"required"`
	NewPassword string `json:"new_password" binding:"required" validate:"required,min=6"`
}

// LoginResponse 登录响应结构
type LoginResponse struct {
	Token        string    `json:"token"`
	User         *AuthUser `json:"user"`
	IsFirstLogin bool      `json:"is_first_login"`
	ExpiresAt    int64     `json:"expires_at"`
}

// VerifyTokenResponse 验证令牌响应结构
type VerifyTokenResponse struct {
	Valid    bool      `json:"valid"`
	User     *AuthUser `json:"user,omitempty"`
	ExpiresAt int64    `json:"expires_at,omitempty"`
}