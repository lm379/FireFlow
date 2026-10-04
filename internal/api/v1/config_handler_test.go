package v1

import (
	"FireFlow/internal/core"
	"FireFlow/internal/logger"
	"FireFlow/internal/model"
	"FireFlow/internal/repository"
	"FireFlow/internal/service"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func setupConfigHandler(t *testing.T) (*gin.Engine, service.ConfigService, *gorm.DB, *core.CronManager) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	logger.InfoLogger = logrus.New()
	logger.ErrorLogger = logrus.New()
	db, err := gorm.Open(sqlite.New(sqlite.Config{DriverName: "sqlite", DSN: filepath.Join(t.TempDir(), "api.db")}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { sqlDB.Close() })
	if err := db.AutoMigrate(&model.ConfigItem{}, &model.CloudProviderConfig{}, &model.FirewallRule{}); err != nil {
		t.Fatal(err)
	}
	config := service.NewConfigService(repository.NewConfigRepository(db))
	firewall := service.NewFirewallService(repository.NewFirewallRepo(db), config)
	cm := core.NewCronManager()
	cm.SetUpdateFunc(func() {})
	handler := NewConfigHandler(config, cm)
	handler.SetFirewallService(firewall)
	router := gin.New()
	router.GET("/config", handler.GetSystemConfig)
	router.PUT("/config", handler.SetSystemConfig)
	router.POST("/sync", handler.SyncIPNow)
	return router, config, db, cm
}

func callConfigAPI(router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestSystemConfigRoundTripKeepsSchedulerEnabled(t *testing.T) {
	router, config, _, cm := setupConfigHandler(t)
	w := callConfigAPI(router, "PUT", "/config", `{"cron_enabled":"true","ip_check_interval":90}`)
	if w.Code != http.StatusOK || !cm.IsRunning() {
		t.Fatalf("save failed: %d %s", w.Code, w.Body)
	}
	w = callConfigAPI(router, "GET", "/config", "")
	var settings map[string]interface{}
	if err := unmarshalResponseData(w.Body.Bytes(), &settings); err != nil {
		t.Fatal(err)
	}
	if settings["ip_check_interval"] != float64(90) {
		t.Fatalf("interval is not numeric: %#v", settings)
	}
	// A settings round trip and a partial update must both retain the job.
	settingsBody, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	w = callConfigAPI(router, "PUT", "/config", string(settingsBody))
	if w.Code != http.StatusOK || !cm.IsRunning() {
		t.Fatalf("round trip stopped job: %s", w.Body)
	}
	w = callConfigAPI(router, "PUT", "/config", `{"ip_fetch_url":"https://example.invalid"}`)
	if w.Code != http.StatusOK || !cm.IsRunning() {
		t.Fatalf("partial update stopped job: %s", w.Body)
	}
	for _, interval := range []string{"0", "-1", "1.5", `"invalid"`} {
		w = callConfigAPI(router, "PUT", "/config", `{"ip_check_interval":`+interval+`}`)
		if w.Code != http.StatusBadRequest || !cm.IsRunning() {
			t.Fatalf("invalid interval %s changed job: %d", interval, w.Code)
		}
		stored, err := config.GetConfigInt("ip_check_interval")
		if err != nil || stored != 90 {
			t.Fatalf("invalid interval saved: %d, %v", stored, err)
		}
	}
}

func TestSyncAPIReportsCloudFailures(t *testing.T) {
	router, config, db, _ := setupConfigHandler(t)
	requests := 0
	ip := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Write([]byte("192.0.2.1"))
	}))
	defer ip.Close()
	if err := config.SetConfig("ip_fetch_url", ip.URL, "string", "system", ""); err != nil {
		t.Fatal(err)
	}
	cloud := model.CloudProviderConfig{Provider: "Azure", IsEnabled: true}
	if err := config.CreateCloudConfig(&cloud); err != nil {
		t.Fatal(err)
	}
	// Explicitly disable after creation to bypass GORM's true default on inserts.
	if err := db.Model(&cloud).Update("is_enabled", false).Error; err != nil {
		t.Fatal(err)
	}
	rule := model.FirewallRule{CloudConfigID: cloud.ID, Enabled: true, Remark: "test", Port: "22"}
	if err := db.Create(&rule).Error; err != nil {
		t.Fatal(err)
	}
	w := callConfigAPI(router, "POST", "/sync", "")
	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusBadGateway || result["code"] != float64(http.StatusBadGateway) || result["data"] != nil || !strings.Contains(result["msg"].(string), "失败 1 条") || requests != 1 {
		t.Fatalf("sync misreported: status=%d body=%s requests=%d", w.Code, w.Body, requests)
	}
}
