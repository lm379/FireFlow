package repository

import (
	"FireFlow/internal/model"
	"gorm.io/gorm"
)

type AuthUserRepository interface {
	GetByUsername(username string) (*model.AuthUser, error)
	GetByID(id uint) (*model.AuthUser, error)
	Create(user *model.AuthUser) error
	Update(user *model.AuthUser) error
	UpdatePassword(id uint, hashedPassword string) error
	UpdateLoginTime(id uint) error
	SetFirstLoginCompleted(id uint) error
	GetUserTokenVersion(userID uint) (int, error)
	IncrementTokenVersion(userID uint) error
}

type authUserRepository struct {
	db *gorm.DB
}

func NewAuthUserRepository(db *gorm.DB) AuthUserRepository {
	return &authUserRepository{
		db: db,
	}
}

func (r *authUserRepository) GetByUsername(username string) (*model.AuthUser, error) {
	var user model.AuthUser
	err := r.db.Where("username = ?", username).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *authUserRepository) GetByID(id uint) (*model.AuthUser, error) {
	var user model.AuthUser
	err := r.db.First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *authUserRepository) Create(user *model.AuthUser) error {
	return r.db.Create(user).Error
}

func (r *authUserRepository) Update(user *model.AuthUser) error {
	return r.db.Save(user).Error
}

func (r *authUserRepository) UpdatePassword(id uint, hashedPassword string) error {
	return r.db.Model(&model.AuthUser{}).Where("id = ?", id).Updates(map[string]interface{}{
		"password":            hashedPassword,
		"password_updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
	}).Error
}

func (r *authUserRepository) UpdateLoginTime(id uint) error {
	return r.db.Model(&model.AuthUser{}).Where("id = ?", id).Update("last_login_time", gorm.Expr("CURRENT_TIMESTAMP")).Error
}

func (r *authUserRepository) SetFirstLoginCompleted(id uint) error {
	return r.db.Model(&model.AuthUser{}).Where("id = ?", id).Update("is_first_login", false).Error
}

func (r *authUserRepository) GetUserTokenVersion(userID uint) (int, error) {
	var user model.AuthUser
	err := r.db.Select("token_version").Where("id = ?", userID).First(&user).Error
	if err != nil {
		return 0, err
	}
	return user.TokenVersion, nil
}

func (r *authUserRepository) IncrementTokenVersion(userID uint) error {
	return r.db.Model(&model.AuthUser{}).Where("id = ?", userID).Update("token_version", gorm.Expr("token_version + 1")).Error
}