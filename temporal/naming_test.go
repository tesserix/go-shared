package temporal

import "testing"

func TestTaskQueue(t *testing.T) {
	if got := TaskQueue("homechef", "orders"); got != "homechef-orders" {
		t.Fatalf("TaskQueue = %q, want homechef-orders", got)
	}
	if got := TaskQueue("Marketplace", "Payments"); got != "marketplace-payments" {
		t.Fatalf("TaskQueue lowercases: got %q", got)
	}
}

func TestWorkflowID(t *testing.T) {
	if got := WorkflowID("homechef", "order", "abc-123"); got != "homechef:order:abc-123" {
		t.Fatalf("WorkflowID = %q, want homechef:order:abc-123", got)
	}
}

func TestConfig_OptIn(t *testing.T) {
	t.Setenv("TEMPORAL_HOSTPORT", "")
	t.Setenv("TEMPORAL_ENABLED", "")
	if LoadConfig().Enabled() {
		t.Fatal("Config should be disabled with no env set")
	}
	t.Setenv("TEMPORAL_HOSTPORT", "temporal-frontend:7233")
	if !LoadConfig().Enabled() {
		t.Fatal("Config should be enabled once TEMPORAL_HOSTPORT is set")
	}
}
