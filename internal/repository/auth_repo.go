package repository

import (
	"FireFlow/internal/model"
	"errors"
	"gorm.io/gorm"
)

type AuthUserRepository interface {
	GetByUsername(username string) (*model.AuthUser, error)
	GetByID(id uint) (*model.AuthUser, error)
	Create(user *model.AuthUser) error
	Update(user *model.AuthUser) error
	ReplacePassword(id uint, expectedVersion int, hashedPassword string, firstLogin bool) error
	UpdateLoginTime(id uint) error
	GetUserTokenVersion(userID uint) (int, error)
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

var ErrCredentialsChanged = errors.New("credentials changed, please log in again")

func (r *authUserRepository) ReplacePassword(id uint, expectedVersion int, hashedPassword string, firstLogin bool) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.AuthUser{}).Where("id = ? AND token_version = ?", id, expectedVersion).Updates(map[string]interface{}{
			"password":            hashedPassword,
			"password_updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrCredentialsChanged
		}
		if err := tx.Model(&model.AuthUser{}).Where("id = ?", id).Update("token_version", gorm.Expr("token_version + 1")).Error; err != nil {
			return err
		}
		return tx.Model(&model.AuthUser{}).Where("id = ?", id).Update("is_first_login", firstLogin).Error
	})
}

func (r *authUserRepository) UpdateLoginTime(id uint) error {
	return r.db.Model(&model.AuthUser{}).Where("id = ?", id).Update("last_login_time", gorm.Expr("CURRENT_TIMESTAMP")).Error
}

func (r *authUserRepository) GetUserTokenVersion(userID uint) (int, error) {
	var user model.AuthUser
	err := r.db.Select("token_version").Where("id = ?", userID).First(&user).Error
	if err != nil {
		return 0, err
	}
	return user.TokenVersion, nil
}
