package core

import (
	"testing"
	"time"
)

func TestFirewallJobUsesConsistentIntervals(t *testing.T) {
	for _, minutes := range []int{45, 90} {
		cm := NewCronManager()
		cm.SetUpdateFunc(func() {})
		if err := cm.StartFirewallUpdateJob(minutes); err != nil {
			t.Fatal(err)
		}
		schedule := cm.cron.Entry(cm.firewallJobID).Schedule
		at := time.Date(2026, 1, 1, 12, 45, 0, 0, time.UTC)
		for range 3 {
			next := schedule.Next(at)
			if next.Sub(at) != time.Duration(minutes)*time.Minute {
				t.Fatalf("interval %d: got %v", minutes, next.Sub(at))
			}
			at = next
		}
		id := cm.firewallJobID
		for _, invalid := range []int{0, -1} {
			if err := cm.StartFirewallUpdateJob(invalid); err == nil {
				t.Fatal("invalid interval accepted")
			}
			if cm.firewallJobID != id || !cm.IsRunning() {
				t.Fatal("invalid interval replaced the active job")
			}
		}
	}
}
