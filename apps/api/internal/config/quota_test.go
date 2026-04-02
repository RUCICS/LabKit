package config

import "testing"

func TestResolveQuotaLocationFromEnvDefaultsToAsiaShanghai(t *testing.T) {
	t.Setenv(quotaTimezoneEnv, "")

	got, err := ResolveQuotaLocationFromEnv()
	if err != nil {
		t.Fatalf("ResolveQuotaLocationFromEnv() error = %v", err)
	}
	if got == nil {
		t.Fatal("ResolveQuotaLocationFromEnv() returned nil location")
	}
	if got.String() != "Asia/Shanghai" {
		t.Fatalf("location = %q, want %q", got.String(), "Asia/Shanghai")
	}
}

func TestResolveQuotaLocationFromEnvRejectsInvalidTimezone(t *testing.T) {
	t.Setenv(quotaTimezoneEnv, "Mars/Olympus")

	_, err := ResolveQuotaLocationFromEnv()
	if err == nil {
		t.Fatal("ResolveQuotaLocationFromEnv() error = nil, want error")
	}
	if err.Error() == "" {
		t.Fatal("ResolveQuotaLocationFromEnv() returned empty error")
	}
}
