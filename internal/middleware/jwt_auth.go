package middleware

import (
	"FireFlow/internal/logger"
	"FireFlow/internal/model"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const (
	// JWT相关常量
	DefaultTokenExpiration = 72 * time.Hour // 默认72小时过期
	TokenIssuer           = "fireflow"
)

var (
	jwtSecret         []byte        // JWT密钥
	tokenExpiration   time.Duration // JWT过期时间
	tokenValidator    TokenValidator // 令牌版本验证器
)

// TokenValidator 令牌版本验证器接口
type TokenValidator interface {
	GetUserTokenVersion(userID uint) (int, error)
}

// SetTokenValidator 设置令牌版本验证器
func SetTokenValidator(validator TokenValidator) {
	tokenValidator = validator
}

// SetJWTSecret 设置JWT密钥
func SetJWTSecret(secret []byte) {
	jwtSecret = secret
}

// SetTokenExpiration 设置JWT过期时间（小时）
func SetTokenExpiration(hours int) {
	if hours <= 0 {
		tokenExpiration = DefaultTokenExpiration
	} else {
		tokenExpiration = time.Duration(hours) * time.Hour
	}
}

// JWTClaims JWT Claims结构
type JWTClaims struct {
	UserID       uint   `json:"user_id"`
	Username     string `json:"username"`
	TokenVersion int    `json:"token_version"`
	jwt.RegisteredClaims
}

// GenerateToken 生成JWT令牌
func GenerateToken(user *model.AuthUser) (string, int64, error) {
	if len(jwtSecret) == 0 {
		return "", 0, fmt.Errorf("JWT secret not configured")
	}

	// 如果没有设置过期时间，使用默认值
	expiration := tokenExpiration
	if expiration == 0 {
		expiration = DefaultTokenExpiration
	}

	expirationTime := time.Now().Add(expiration)

	claims := &JWTClaims{
		UserID:       user.ID,
		Username:     user.Username,
		TokenVersion: user.TokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    TokenIssuer,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		return "", 0, err
	}

	return tokenString, expirationTime.Unix(), nil
}

// ValidateToken 验证JWT令牌
func ValidateToken(tokenString string) (*JWTClaims, error) {
	if len(jwtSecret) == 0 {
		return nil, fmt.Errorf("JWT secret not configured")
	}

	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		// 验证令牌版本
		if tokenValidator != nil {
			currentVersion, err := tokenValidator.GetUserTokenVersion(claims.UserID)
			if err != nil {
				logger.ErrorLogger.Warnf("Failed to get user token version: %v", err)
				return nil, fmt.Errorf("token validation failed")
			}
			
			if claims.TokenVersion != currentVersion {
				logger.InfoLogger.Infof("Token version mismatch for user %d: token=%d, current=%d", 
					claims.UserID, claims.TokenVersion, currentVersion)
				return nil, fmt.Errorf("token has been invalidated")
			}
		}
		
		return claims, nil
	}

	return nil, jwt.ErrInvalidKey
}

// JWTAuthMiddleware JWT认证中间件
func JWTAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从请求头获取token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Authorization header is required",
				"code":    "MISSING_AUTH_HEADER",
			})
			c.Abort()
			return
		}

		// 检查Bearer前缀
		tokenParts := strings.SplitN(authHeader, " ", 2)
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Invalid authorization header format",
				"code":    "INVALID_AUTH_HEADER",
			})
			c.Abort()
			return
		}

		tokenString := tokenParts[1]

		// 验证token
		claims, err := ValidateToken(tokenString)
		if err != nil {
			logger.InfoLogger.Warnf("Invalid token: %v", err)

			var message string
			var code string

			if err == jwt.ErrTokenExpired {
				message = "Token has expired"
				code = "TOKEN_EXPIRED"
			} else if err == jwt.ErrTokenMalformed {
				message = "Token is malformed"
				code = "TOKEN_MALFORMED"
			} else if err == jwt.ErrSignatureInvalid {
				message = "Token signature is invalid"
				code = "TOKEN_INVALID_SIGNATURE"
			} else {
				message = "Invalid token"
				code = "TOKEN_INVALID"
			}

			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": message,
				"code":    code,
			})
			c.Abort()
			return
		}

		// 将用户信息存储到context中
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("jwt_claims", claims)

		c.Next()
	}
}

// GetCurrentUserID 从context中获取当前用户ID
func GetCurrentUserID(c *gin.Context) (uint, bool) {
	userID, exists := c.Get("user_id")
	if !exists {
		return 0, false
	}

	if id, ok := userID.(uint); ok {
		return id, true
	}

	return 0, false
}

// GetCurrentUsername 从context中获取当前用户名
func GetCurrentUsername(c *gin.Context) (string, bool) {
	username, exists := c.Get("username")
	if !exists {
		return "", false
	}

	if name, ok := username.(string); ok {
		return name, true
	}

	return "", false
}

// OptionalJWTAuthMiddleware 可选的JWT认证中间件（不强制要求认证）
func OptionalJWTAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		tokenParts := strings.SplitN(authHeader, " ", 2)
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			c.Next()
			return
		}

		tokenString := tokenParts[1]
		claims, err := ValidateToken(tokenString)
		if err != nil {
			c.Next()
			return
		}

		// 将用户信息存储到context中
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("jwt_claims", claims)

		c.Next()
	}
}
