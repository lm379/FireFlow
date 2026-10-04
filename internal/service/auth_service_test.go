package service

import (
	"FireFlow/internal/middleware"
	"FireFlow/internal/model"
	"FireFlow/internal/repository"
	"encoding/base64"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func newTestAuthService(t *testing.T) (AuthService, repository.AuthUserRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.New(sqlite.Config{DriverName: "sqlite", DSN: filepath.Join(t.TempDir(), "auth.db")}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	conn, _ := db.DB()
	t.Cleanup(func() { conn.Close(); middleware.SetTokenValidator(nil); middleware.SetJWTSecret(nil) })
	if err := db.AutoMigrate(&model.AuthUser{}); err != nil {
		t.Fatal(err)
	}
	repo := repository.NewAuthUserRepository(db)
	svc := NewAuthService(repo)
	middleware.SetJWTSecret([]byte("auth-lifecycle-test-signing-key"))
	middleware.SetTokenValidator(svc)
	return svc, repo, db
}

func TestTemporaryPasswordLifecycle(t *testing.T) {
	svc, repo, _ := newTestAuthService(t)
	temporary, err := svc.InitializeDefaultUser()
	if err != nil {
		t.Fatal(err)
	}
	entropy, err := base64.RawURLEncoding.DecodeString(temporary)
	if err != nil || len(entropy) != 24 {
		t.Fatal("invalid temporary password")
	}
	user, _ := repo.GetByUsername(DefaultUsername)
	if !user.IsFirstLogin || bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(temporary)) != nil {
		t.Fatal("invalid initial credentials")
	}
	version := user.TokenVersion
	if repeated, err := svc.InitializeDefaultUser(); err != nil || repeated != "" {
		t.Fatal("existing credentials were regenerated")
	}
	if _, err := svc.Login(DefaultUsername, "password"); err == nil {
		t.Fatal("fixed default password accepted")
	}
	login, err := svc.Login(DefaultUsername, temporary)
	if err != nil || !login.IsFirstLogin {
		t.Fatalf("temporary login failed: %v", err)
	}
	if err := svc.ChangePassword(user.ID, temporary, temporary); err == nil {
		t.Fatal("temporary password reused")
	}
	if err := svc.ChangePassword(user.ID, temporary, "new-test-password"); err != nil {
		t.Fatal(err)
	}
	if _, err := middleware.ValidateToken(login.Token); err == nil {
		t.Fatal("temporary token not revoked")
	}
	user, _ = repo.GetByID(user.ID)
	if user.IsFirstLogin || user.TokenVersion != version+1 {
		t.Fatal("password change state incorrect")
	}
	login, err = svc.Login(DefaultUsername, "new-test-password")
	if err != nil || login.IsFirstLogin {
		t.Fatalf("normal login failed: %v", err)
	}
	reset, err := svc.ResetAdminPassword()
	if err != nil || reset == temporary || len(reset) != 32 {
		t.Fatal("reset did not generate fresh credentials")
	}
	if _, err := middleware.ValidateToken(login.Token); err == nil {
		t.Fatal("reset did not revoke token")
	}
	if _, err := svc.Login(DefaultUsername, "new-test-password"); err == nil {
		t.Fatal("old password still valid")
	}
	login, err = svc.Login(DefaultUsername, reset)
	if err != nil || !login.IsFirstLogin {
		t.Fatalf("reset login failed: %v", err)
	}
}

func TestPasswordFailuresDoNotExposeOrCommitNewCredentials(t *testing.T) {
	svc, repo, db := newTestAuthService(t)
	temporary, err := svc.ResetAdminPassword()
	if err != nil || temporary == "" {
		t.Fatal("reset did not initialize missing admin")
	}
	before, _ := repo.GetByUsername(DefaultUsername)
	if err := db.Exec("CREATE TRIGGER fail_version BEFORE UPDATE OF token_version ON auth_users BEGIN SELECT RAISE(ABORT, 'injected failure'); END").Error; err != nil {
		t.Fatal(err)
	}
	if password, err := svc.ResetAdminPassword(); err == nil || password != "" {
		t.Fatal("failed reset returned new credentials")
	}
	if err := svc.ChangePassword(before.ID, temporary, "new-test-password"); err == nil {
		t.Fatal("failed change reported success")
	}
	after, _ := repo.GetByID(before.ID)
	if after.Password != before.Password || after.TokenVersion != before.TokenVersion || after.IsFirstLogin != before.IsFirstLogin {
		t.Fatal("failed operation partially committed")
	}
}

type loginSnapshotRepo struct {
	repository.AuthUserRepository
	user    *model.AuthUser
	lookups int
}

func (r *loginSnapshotRepo) GetByUsername(string) (*model.AuthUser, error) { return r.user, nil }
func (r *loginSnapshotRepo) GetByID(uint) (*model.AuthUser, error) {
	r.lookups++
	newer := *r.user
	newer.TokenVersion++
	return &newer, nil
}
func (r *loginSnapshotRepo) UpdateLoginTime(uint) error { return nil }

func TestLoginTokenUsesVersionOfVerifiedPassword(t *testing.T) {
	_, _, _ = newTestAuthService(t)
	hash, err := bcrypt.GenerateFromPassword([]byte("test-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	repo := &loginSnapshotRepo{user: &model.AuthUser{ID: 1, Username: "test", Password: string(hash), TokenVersion: 4}}
	middleware.SetTokenValidator(nil)
	login, err := NewAuthService(repo).Login("test", "test-password")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := middleware.ValidateToken(login.Token)
	if err != nil || claims.TokenVersion != 4 || repo.lookups != 0 {
		t.Fatal("login used a version not belonging to the verified password")
	}
}
