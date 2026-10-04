package cloud

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	lighthouse "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/lighthouse/v20200324"
)

func TestTencentSynchronizationReportsRemoteChanges(t *testing.T) {
	for _, previous := range []string{"192.0.2.1/32", "203.0.113.2/32", ""} {
		t.Run("previous="+previous, func(t *testing.T) {
			remoteCIDR := previous
			creates, deletes := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				response := map[string]interface{}{"RequestId": "test"}
				switch r.Header.Get("X-TC-Action") {
				case "DescribeFirewallRules":
					rules := []map[string]string{}
					if remoteCIDR != "" {
						rules = append(rules, map[string]string{"Port": "22", "Protocol": "TCP", "Action": "ACCEPT", "CidrBlock": remoteCIDR, "FirewallRuleDescription": "sync-test"})
					}
					response["FirewallRuleSet"] = rules
				case "DeleteFirewallRules":
					deletes++
					remoteCIDR = ""
				case "CreateFirewallRules":
					var request struct{ FirewallRules []struct{ CidrBlock string } }
					if err := json.NewDecoder(r.Body).Decode(&request); err != nil || len(request.FirewallRules) != 1 {
						t.Errorf("invalid create request: %v", err)
						w.WriteHeader(400)
						return
					}
					creates++
					remoteCIDR = request.FirewallRules[0].CidrBlock
				default:
					t.Errorf("unexpected action: %s", r.Header.Get("X-TC-Action"))
					w.WriteHeader(400)
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"Response": response})
			}))
			defer server.Close()
			cp := profile.NewClientProfile()
			cp.HttpProfile.Endpoint = strings.TrimPrefix(server.URL, "http://")
			cp.HttpProfile.Scheme = "HTTP"
			sdk, err := lighthouse.NewClient(common.NewCredential("test-ak", "test-sk"), "ap-beijing", cp)
			if err != nil {
				t.Fatal(err)
			}
			client := &TencentClient{lighthouseClient: sdk}
			spec := &FirewallRuleSpec{Port: "22", Protocol: "TCP", Action: "ACCEPT", CidrBlock: "203.0.113.2/32", Description: "sync-test"}
			result, err := client.CreateFirewallRule("test-instance", spec)
			if err != nil {
				t.Fatal(err)
			}
			changed := previous != spec.CidrBlock
			if result.Changed != changed || result.CidrBlock != spec.CidrBlock || remoteCIDR != spec.CidrBlock {
				t.Fatalf("unexpected synchronization result: %#v, remote=%s", result, remoteCIDR)
			}
			if changed && result.PreviousCidrBlock != previous {
				t.Fatalf("previous CIDR = %s, want %s", result.PreviousCidrBlock, previous)
			}
			wantCreates, wantDeletes := 0, 0
			if changed {
				wantCreates = 1
				if previous != "" {
					wantDeletes = 1
				}
			}
			if creates != wantCreates || deletes != wantDeletes {
				t.Fatalf("creates=%d deletes=%d", creates, deletes)
			}
			result, err = client.CreateFirewallRule("test-instance", spec)
			if err != nil || result.Changed || creates != wantCreates || deletes != wantDeletes {
				t.Fatalf("repeat synchronization: result=%#v err=%v creates=%d deletes=%d", result, err, creates, deletes)
			}
		})
	}
}
