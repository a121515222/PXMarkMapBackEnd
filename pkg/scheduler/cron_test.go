package scheduler

import "testing"

func TestGetRetentionDaysUsesEnvValue(t *testing.T) {
	t.Setenv("SHIPMENT_RETENTION_DAYS", "45")

	if got := getRetentionDays(); got != 45 {
		t.Fatalf("expected 45 from env, got %d", got)
	}
}

func TestGetRetentionDaysUsesDefaultWhenUnsetOrInvalid(t *testing.T) {
	t.Setenv("SHIPMENT_RETENTION_DAYS", "")
	if got := getRetentionDays(); got != 30 {
		t.Fatalf("expected default 30 when env is unset, got %d", got)
	}

	t.Setenv("SHIPMENT_RETENTION_DAYS", "abc")
	if got := getRetentionDays(); got != 30 {
		t.Fatalf("expected default 30 when env is invalid, got %d", got)
	}
}
