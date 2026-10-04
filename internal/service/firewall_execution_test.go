package service

import (
	"FireFlow/pkg/cloud"
	"testing"
)

func TestFirewallExecutionResult(t *testing.T) {
	const currentIP = "203.0.113.2"
	tests := []struct {
		name      string
		remote    *cloud.FirewallRuleResult
		status    string
		previous  string
		wantError bool
	}{
		{"updated remote IP", &cloud.FirewallRuleResult{Changed: true, PreviousCidrBlock: "192.0.2.1/32", CidrBlock: currentIP + "/32"}, "updated", "192.0.2.1", false},
		{"already synchronized", &cloud.FirewallRuleResult{CidrBlock: currentIP + "/32"}, "unchanged", "", false},
		{"created remote rule", &cloud.FirewallRuleResult{Changed: true, CidrBlock: currentIP + "/32"}, "updated", "", false},
		{"missing result", nil, "", "", true},
		{"missing remote IP", &cloud.FirewallRuleResult{Changed: true}, "", "", true},
		{"remote IP differs after update", &cloud.FirewallRuleResult{Changed: true, CidrBlock: "192.0.2.1/32"}, "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := firewallExecutionResult(currentIP, tt.remote)
			if (err != nil) != tt.wantError {
				t.Fatalf("error = %v", err)
			}
			if tt.wantError {
				return
			}
			if result["status"] != tt.status || result["previous_ip"] != tt.previous || result["cloud_ip"] != currentIP || result["current_ip"] != currentIP || result["ip_changed"] != tt.remote.Changed {
				t.Fatalf("unexpected result: %#v", result)
			}
		})
	}
}
