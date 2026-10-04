package v1

import (
	"FireFlow/internal/dto"
	"FireFlow/internal/model"
	"FireFlow/internal/repository"
	"FireFlow/internal/service"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func assertNoSensitiveFields(t *testing.T, body string) {
	t.Helper()
	var value any
	if err := json.Unmarshal([]byte(body), &value); err != nil {
		t.Fatal(err)
	}
	var inspect func(any)
	inspect = func(value any) {
		switch value := value.(type) {
		case map[string]any:
			for key, item := range value {
				switch strings.ToLower(key) {
				case "secret_id", "secret_key", "extra", "password", "token_version", "password_updated_at", "deletedat", "cloud_config":
					t.Errorf("sensitive/internal field %q in response: %s", key, body)
				}
				inspect(item)
			}
		case []any:
			for _, item := range value {
				inspect(item)
			}
		}
	}
	inspect(value)
	for _, secret := range []string{"private-ak", "private-sk", "private-extra", "private-password"} {
		if strings.Contains(body, secret) {
			t.Errorf("secret value in response: %s", body)
		}
	}
}

func TestCloudDTOResponsesAndCredentialUpdates(t *testing.T) {
	router, config, db, _ := setupConfigHandler(t)
	h := NewCloudConfigHandler(config)
	router.GET("/clouds", h.GetCloudConfigs)
	router.GET("/clouds/:id", h.GetCloudConfig)
	router.POST("/clouds", h.CreateCloudConfig)
	router.PUT("/clouds/:id", h.UpdateCloudConfig)
	w := callConfigAPI(router, "POST", "/clouds", `{"ID":999,"provider":"Azure","secret_id":"private-ak","secret_key":"private-sk","extra":"private-extra","instance_id":"nsg","tenant_id":"tenant","subscription_id":"sub","is_enabled":true}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", w.Code, w.Body)
	}
	assertNoSensitiveFields(t, w.Body.String())
	var created dto.CloudConfigResponse
	if err := unmarshalResponseData(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID == 0 || created.ID == 999 {
		t.Fatal("client supplied the database ID")
	}
	path := fmt.Sprintf("/clouds/%d", created.ID)
	for _, path := range []string{"/clouds", path} {
		w = callConfigAPI(router, "GET", path, "")
		if w.Code != http.StatusOK {
			t.Fatalf("read: %d %s", w.Code, w.Body)
		}
		assertNoSensitiveFields(t, w.Body.String())
	}
	for _, body := range []string{`{"description":"edited","is_enabled":false}`, `{"secret_id":"","secret_key":""}`} {
		w = callConfigAPI(router, "PUT", path, body)
		if w.Code != http.StatusOK {
			t.Fatalf("update: %d %s", w.Code, w.Body)
		}
		assertNoSensitiveFields(t, w.Body.String())
		stored, err := config.GetCloudConfigByID(created.ID)
		if err != nil {
			t.Fatal(err)
		}
		if stored.SecretId != "private-ak" || stored.SecretKey != "private-sk" || stored.Extra != "private-extra" || stored.TenantID != "tenant" || stored.IsEnabled {
			t.Fatalf("omitted fields or credentials were overwritten: %#v", stored)
		}
	}
	w = callConfigAPI(router, "PUT", path, `{"secret_id":"replacement-ak","secret_key":"replacement-sk"}`)
	if w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
	assertNoSensitiveFields(t, w.Body.String())
	stored, _ := config.GetCloudConfigByID(created.ID)
	if stored.SecretId != "replacement-ak" || stored.SecretKey != "replacement-sk" {
		t.Fatal("credential replacement failed")
	}
	w = callConfigAPI(router, "PUT", path, `{"provider":"Aliyun"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatal("provider change reused old credentials")
	}

	firewall := NewFirewallHandler(service.NewFirewallService(repository.NewFirewallRepo(db), config))
	firewall.SetConfigService(config)
	router.POST("/rules", firewall.CreateRule)
	router.GET("/rules", firewall.GetRules)
	router.GET("/rules/:id", firewall.GetRule)
	router.PUT("/rules/:id", firewall.UpdateRule)
	body := fmt.Sprintf(`{"cloud_config_id":%d,"port":"22","protocol":"TCP","remark":"test","enabled":true,"cloud_config":{"secret_key":"private-sk"}}`, created.ID)
	w = callConfigAPI(router, "POST", "/rules", body)
	if w.Code != http.StatusCreated {
		t.Fatalf("create rule: %d %s", w.Code, w.Body)
	}
	assertNoSensitiveFields(t, w.Body.String())
	var rule dto.FirewallRuleResponse
	if err := unmarshalResponseData(w.Body.Bytes(), &rule); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/rules", fmt.Sprintf("/rules/%d", rule.ID)} {
		w = callConfigAPI(router, "GET", path, "")
		if w.Code != http.StatusOK {
			t.Fatal(w.Body.String())
		}
		assertNoSensitiveFields(t, w.Body.String())
	}
	w = callConfigAPI(router, "PUT", fmt.Sprintf("/rules/%d", rule.ID), body)
	if w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
	assertNoSensitiveFields(t, w.Body.String())
	var count int64
	db.Model(&model.CloudProviderConfig{}).Count(&count)
	if count != 1 {
		t.Fatal("nested input created an extra cloud configuration")
	}
}

func TestGenericConfigResponsesDoNotExposePrivateValues(t *testing.T) {
	router, config, _, cm := setupConfigHandler(t)
	h := NewConfigHandler(config, cm)
	router.GET("/values/:key", h.GetConfig)
	router.PUT("/values/:key", h.SetConfig)
	router.GET("/values", h.GetConfigs)
	router.GET("/category/:category", h.GetConfigsByCategory)
	for _, key := range []string{"jwt_secret", "secret_key", "custom_credentials"} {
		w := callConfigAPI(router, "PUT", "/values/"+key, `{"value":"private-sk","category":"system"}`)
		if w.Code != http.StatusOK {
			t.Fatal(w.Body.String())
		}
		assertNoSensitiveFields(t, w.Body.String())
		w = callConfigAPI(router, "GET", "/values/"+key, "")
		if w.Code != http.StatusOK {
			t.Fatal(w.Body.String())
		}
		assertNoSensitiveFields(t, w.Body.String())
	}
	for _, path := range []string{"/values?category=system", "/category/system", "/config"} {
		w := callConfigAPI(router, "GET", path, "")
		if w.Code != http.StatusOK {
			t.Fatal(w.Body.String())
		}
		// Key names are metadata; values and system-only responses must be hidden.
		if strings.Contains(w.Body.String(), "private-sk") {
			t.Fatalf("private config leaked: %s", w.Body)
		}
		if path == "/config" && strings.Contains(w.Body.String(), "jwt_secret") {
			t.Fatal("system response included private setting")
		}
	}
}

type authDTOStub struct {
	service.AuthService
	user *model.AuthUser
}

func (s authDTOStub) GetUserByID(uint) (*model.AuthUser, error) { return s.user, nil }
func (s authDTOStub) Login(string, string) (*model.LoginResponse, error) {
	return &model.LoginResponse{Token: "login-token", User: s.user}, nil
}
func (s authDTOStub) VerifyToken(string) (*model.VerifyTokenResponse, error) {
	return &model.VerifyTokenResponse{Valid: true, User: s.user}, nil
}

func TestAuthResponsesUsePublicUserDTO(t *testing.T) {
	router, _, _, _ := setupConfigHandler(t)
	now := time.Now()
	h := NewAuthHandler(authDTOStub{user: &model.AuthUser{ID: 1, Username: "admin", Password: "private-password", TokenVersion: 7, PasswordUpdatedAt: &now}})
	router.POST("/login", h.Login)
	router.GET("/verify", h.VerifyToken)
	router.GET("/me", func(c *gin.Context) { c.Set("user_id", uint(1)); h.GetCurrentUser(c) })
	for _, req := range []struct{ method, path, body string }{
		{"POST", "/login", `{"username":"admin","password":"password"}`},
		{"GET", "/verify?token=test", ""}, {"GET", "/me", ""},
	} {
		w := callConfigAPI(router, req.method, req.path, req.body)
		if w.Code != http.StatusOK {
			t.Fatal(w.Body.String())
		}
		assertNoSensitiveFields(t, w.Body.String())
		if !strings.Contains(w.Body.String(), "admin") {
			t.Fatal("public username missing")
		}
	}
}
