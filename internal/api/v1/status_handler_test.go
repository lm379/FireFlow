package v1

import (
	"FireFlow/internal/model"
	"FireFlow/internal/repository"
	"FireFlow/internal/service"
	"fmt"
	"net/http"
	"reflect"
	"testing"
)

func TestStatusActionsOnlyChangeEnabledState(t *testing.T) {
	router, configs, db, _ := setupConfigHandler(t)
	cloud := model.CloudProviderConfig{Provider: "TencentCloud", SecretId: "test-ak", SecretKey: "test-sk", InstanceId: "instance", Region: "region", IsDefault: true, IsEnabled: true}
	other := model.CloudProviderConfig{Provider: "Azure", SecretId: "other-ak", SecretKey: "other-sk", IsDefault: true, IsEnabled: true}
	for _, config := range []*model.CloudProviderConfig{&cloud, &other} {
		if err := db.Create(config).Error; err != nil {
			t.Fatal(err)
		}
	}
	rule := model.FirewallRule{CloudConfigID: cloud.ID, Port: "22", Protocol: "TCP", Remark: "ssh", LastIP: "192.0.2.1", Enabled: true}
	if err := db.Create(&rule).Error; err != nil {
		t.Fatal(err)
	}
	firewall := NewFirewallHandler(service.NewFirewallService(repository.NewFirewallRepo(db), configs))
	cloudHandler := NewCloudConfigHandler(configs)
	router.POST("/rules/:id/status", firewall.SetStatus)
	router.POST("/cloud-configs/:id/status", cloudHandler.SetStatus)
	if err := db.First(&rule, rule.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&cloud, cloud.ID).Error; err != nil {
		t.Fatal(err)
	}
	for _, enabled := range []bool{false, false, true, true} {
		action := "disable"
		if enabled {
			action = "enable"
		}
		for _, path := range []string{fmt.Sprintf("/rules/%d/status", rule.ID), fmt.Sprintf("/cloud-configs/%d/status", cloud.ID)} {
			// Extra editing fields cannot modify either resource through this endpoint.
			body := fmt.Sprintf(`{"action":%q,"port":"80","remark":"overwrite","cloud_config_id":999,"provider":"Aliyun","secret_key":"overwrite","is_default":false}`, action)
			w := callConfigAPI(router, "POST", path, body)
			if w.Code != http.StatusOK {
				t.Fatalf("%s: %d %s", path, w.Code, w.Body)
			}
			var result map[string]interface{}
			if err := unmarshalResponseData(w.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			field := "enabled"
			if path[1] == 'c' {
				field = "is_enabled"
			}
			if result[field] != enabled {
				t.Fatalf("wrong state: %s", w.Body)
			}
			assertNoSensitiveFields(t, w.Body.String())
		}
		var storedRule model.FirewallRule
		var storedCloud, storedOther model.CloudProviderConfig
		if err := db.First(&storedRule, rule.ID).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.First(&storedCloud, cloud.ID).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.First(&storedOther, other.ID).Error; err != nil {
			t.Fatal(err)
		}
		storedRule.UpdatedAt, storedCloud.UpdatedAt = rule.UpdatedAt, cloud.UpdatedAt
		rule.Enabled, cloud.IsEnabled = enabled, enabled
		if !reflect.DeepEqual(storedRule, rule) || !reflect.DeepEqual(storedCloud, cloud) || !storedOther.IsDefault {
			t.Fatal("status action modified rule content, credentials, or default configuration")
		}
	}
	for _, prefix := range []string{"/rules", "/cloud-configs"} {
		for _, body := range []string{`{}`, `{"action":"enanle"}`, `{"action":"test"}`, `{"action":true}`, `{`} {
			w := callConfigAPI(router, "POST", prefix+"/1/status", body)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("invalid action accepted: %d %s", w.Code, w.Body)
			}
		}
		for _, id := range []string{"0", "invalid", "9999"} {
			w := callConfigAPI(router, "POST", prefix+"/"+id+"/status", `{"action":"enable"}`)
			want := http.StatusBadRequest
			if id == "9999" {
				want = http.StatusNotFound
			}
			if w.Code != want {
				t.Fatalf("invalid ID %s: %d %s", id, w.Code, w.Body)
			}
		}
	}
}
