package repository

import (
	"FireFlow/internal/model"
	"errors"
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestPasswordReplacementIsAtomic(t *testing.T) {
	for _, failColumn := range []string{"", "token_version", "is_first_login"} {
		t.Run("failure="+failColumn, func(t *testing.T) {
			db, err := gorm.Open(sqlite.New(sqlite.Config{DriverName: "sqlite", DSN: filepath.Join(t.TempDir(), "auth.db")}), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			conn, _ := db.DB()
			t.Cleanup(func() { conn.Close() })
			if err := db.AutoMigrate(&model.AuthUser{}); err != nil {
				t.Fatal(err)
			}
			repo := NewAuthUserRepository(db)
			user := &model.AuthUser{Username: "test", Password: "old-hash", TokenVersion: 7, IsFirstLogin: true}
			if err := repo.Create(user); err != nil {
				t.Fatal(err)
			}
			if failColumn != "" {
				if err := db.Exec("CREATE TRIGGER fail_auth_update BEFORE UPDATE OF " + failColumn + " ON auth_users BEGIN SELECT RAISE(ABORT, 'injected failure'); END").Error; err != nil {
					t.Fatal(err)
				}
			}
			err = repo.ReplacePassword(user.ID, 7, "new-hash", false)
			stored, readErr := repo.GetByID(user.ID)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if failColumn != "" {
				if err == nil || stored.Password != user.Password || stored.TokenVersion != 7 || !stored.IsFirstLogin || stored.PasswordUpdatedAt != nil || !stored.UpdatedAt.Equal(user.UpdatedAt) {
					t.Fatalf("partial commit: %#v err=%v", stored, err)
				}
				return
			}
			if err != nil || stored.Password != "new-hash" || stored.TokenVersion != 8 || stored.IsFirstLogin || stored.PasswordUpdatedAt == nil {
				t.Fatalf("unexpected state: %#v err=%v", stored, err)
			}
			if err := repo.ReplacePassword(user.ID, 7, "stale-hash", true); !errors.Is(err, ErrCredentialsChanged) {
				t.Fatalf("stale update accepted: %v", err)
			}
			stored, _ = repo.GetByID(user.ID)
			if stored.Password != "new-hash" || stored.TokenVersion != 8 || stored.IsFirstLogin {
				t.Fatal("stale write changed credentials")
			}
			if err := repo.ReplacePassword(user.ID, 8, "reset-hash", true); err != nil {
				t.Fatal(err)
			}
			stored, _ = repo.GetByID(user.ID)
			if stored.Password != "reset-hash" || stored.TokenVersion != 9 || !stored.IsFirstLogin {
				t.Fatal("reset did not require password change")
			}
		})
	}
}
