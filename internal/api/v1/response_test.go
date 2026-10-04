package v1

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

func unmarshalResponseData(body []byte, target interface{}) error {
	var envelope struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
		Msg  string          `json:"msg"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return err
	}
	if envelope.Code != 200 || envelope.Msg == "" || len(envelope.Data) == 0 {
		return fmt.Errorf("invalid success response: %s", body)
	}
	return json.Unmarshal(envelope.Data, target)
}

func TestLookupResponsesHaveOneEnvelope(t *testing.T) {
	router, _, _, _ := setupConfigHandler(t)
	regions := NewRegionHandler()
	services := NewServiceTypeHandler()
	router.GET("/regions", regions.GetRegions)
	router.GET("/search", regions.SearchRegions)
	router.GET("/providers", regions.GetProviders)
	router.GET("/providers/:provider/service-types", services.GetServiceTypesByProvider)
	for _, path := range []string{"/providers", "/regions?provider=Aliyun", "/search?provider=Aliyun&search=invalid-region", "/providers/Aliyun/service-types"} {
		w := callConfigAPI(router, "GET", path, "")
		if w.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body)
		}
		var body map[string]json.RawMessage
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if len(body) != 3 || string(body["code"]) != "200" || len(body["msg"]) == 0 || len(body["data"]) == 0 {
			t.Fatalf("inconsistent response: %s", w.Body)
		}
		if path == "/search?provider=Aliyun&search=invalid-region" {
			var page struct {
				Items []interface{} `json:"items"`
				Total int           `json:"total"`
			}
			if err := unmarshalResponseData(w.Body.Bytes(), &page); err != nil || page.Items == nil || len(page.Items) != 0 || page.Total != 0 {
				t.Fatalf("invalid empty page: %s, %v", w.Body, err)
			}
		}
	}
	for _, path := range []string{"/regions", "/providers/unknown/service-types"} {
		w := callConfigAPI(router, "GET", path, "")
		var body map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if w.Code < 400 || len(body) != 3 || body["code"] != float64(w.Code) || body["data"] != nil || body["msg"] == "" {
			t.Fatalf("invalid error response: %s", w.Body)
		}
	}
}
