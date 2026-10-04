package v1

import (
	"FireFlow/internal/dto"
	"FireFlow/internal/model"
	"FireFlow/internal/repository"
	"FireFlow/internal/service"
	"fmt"
	"net/http"
	"testing"
)

func TestRulesUseCloudReferenceAndLatestConfig(t *testing.T) {
	router, configService, db, _ := setupConfigHandler(t)
	config := model.CloudProviderConfig{Provider: "Azure", InstanceId: "nsg", ProjectID: "project", SecretId: "private-ak", SecretKey: "private-sk"}
	if err := configService.CreateCloudConfig(&config); err != nil {
		t.Fatal(err)
	}
	h := NewFirewallHandler(service.NewFirewallService(repository.NewFirewallRepo(db), configService))
	h.SetConfigService(configService)
	router.POST("/rules", h.CreateRule)
	router.PUT("/rules/:id", h.UpdateRule)
	router.GET("/rules", h.GetRules)
	router.GET("/rules/:id", h.GetRule)
	for _, body := range []string{`{"port":"22","remark":"missing"}`, `{"cloud_config_id":9999,"port":"22","remark":"unknown"}`} {
		if w := callConfigAPI(router, "POST", "/rules", body); w.Code != http.StatusBadRequest {
			t.Fatalf("invalid reference accepted: %d %s", w.Code, w.Body)
		}
	}
	body := fmt.Sprintf(`{"cloud_config_id":%d,"provider":"Aliyun","instance_id":"wrong","project_id":"wrong","port":"22","remark":"ssh"}`, config.ID)
	w := callConfigAPI(router, "POST", "/rules", body)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", w.Code, w.Body)
	}
	var created dto.FirewallRuleResponse
	if err := unmarshalResponseData(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Provider != "Azure" || created.InstanceID != "nsg" || created.ProjectID != "project" {
		t.Fatalf("client metadata overrode reference: %#v", created)
	}
	config.InstanceId, config.ProjectID = "new-nsg", "new-project"
	if err := configService.UpdateCloudConfig(&config); err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf("/rules/%d", created.ID)
	for _, endpoint := range []string{"/rules", path} {
		w = callConfigAPI(router, "GET", endpoint, "")
		if w.Code != http.StatusOK {
			t.Fatal(w.Body.String())
		}
		assertNoSensitiveFields(t, w.Body.String())
		var responses []dto.FirewallRuleResponse
		if endpoint == "/rules" {
			if err := unmarshalResponseData(w.Body.Bytes(), &responses); err != nil {
				t.Fatal(err)
			}
		} else {
			var response dto.FirewallRuleResponse
			if err := unmarshalResponseData(w.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			responses = append(responses, response)
		}
		if len(responses) != 1 || responses[0].InstanceID != "new-nsg" || responses[0].ProjectID != "new-project" {
			t.Fatalf("stale metadata: %s", w.Body)
		}
	}
	if w := callConfigAPI(router, "PUT", path, `{"port":"22","remark":"ssh"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("update without reference accepted: %d %s", w.Code, w.Body)
	}
}
