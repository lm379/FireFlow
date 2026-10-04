package dto

import (
	"FireFlow/internal/model"
	"time"
)

type UserResponse struct {
	ID            uint       `json:"id"`
	Username      string     `json:"username"`
	IsFirstLogin  bool       `json:"is_first_login"`
	LastLoginTime *time.Time `json:"last_login_time"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func User(user *model.AuthUser) *UserResponse {
	if user == nil {
		return nil
	}
	return &UserResponse{ID: user.ID, Username: user.Username, IsFirstLogin: user.IsFirstLogin,
		LastLoginTime: user.LastLoginTime, CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt}
}

type LoginResponse struct {
	Token        string        `json:"token"`
	User         *UserResponse `json:"user"`
	IsFirstLogin bool          `json:"is_first_login"`
	ExpiresAt    int64         `json:"expires_at"`
}

func Login(response *model.LoginResponse) LoginResponse {
	return LoginResponse{Token: response.Token, User: User(response.User),
		IsFirstLogin: response.IsFirstLogin, ExpiresAt: response.ExpiresAt}
}

type VerifyTokenResponse struct {
	Valid     bool          `json:"valid"`
	User      *UserResponse `json:"user,omitempty"`
	ExpiresAt int64         `json:"expires_at,omitempty"`
}

func VerifyToken(response *model.VerifyTokenResponse) VerifyTokenResponse {
	return VerifyTokenResponse{Valid: response.Valid, User: User(response.User), ExpiresAt: response.ExpiresAt}
}
