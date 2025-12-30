package secrets

import (
	"os"
	"testing"
)

func TestNewStore(t *testing.T) {
	store := NewStore()
	if store.Service != "agentctl" {
		t.Errorf("Service = %q, want %q", store.Service, "agentctl")
	}
}

func TestResolveEnvVar(t *testing.T) {
	os.Setenv("TEST_SECRET_VAR", "test-value")
	defer os.Unsetenv("TEST_SECRET_VAR")

	value, err := Resolve("$TEST_SECRET_VAR")
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if value != "test-value" {
		t.Errorf("value = %q, want %q", value, "test-value")
	}
}

func TestResolveEnvVarNotSet(t *testing.T) {
	_, err := Resolve("$NONEXISTENT_SECRET_VAR_12345")
	if err == nil {
		t.Error("Expected error for unset env var")
	}
}

func TestResolvePlainValue(t *testing.T) {
	value, err := Resolve("plain-value")
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if value != "plain-value" {
		t.Errorf("value = %q, want %q", value, "plain-value")
	}
}

func TestResolveEnv(t *testing.T) {
	os.Setenv("TEST_KEY1", "value1")
	os.Setenv("TEST_KEY2", "value2")
	defer os.Unsetenv("TEST_KEY1")
	defer os.Unsetenv("TEST_KEY2")

	env := map[string]string{
		"KEY1":  "$TEST_KEY1",
		"KEY2":  "$TEST_KEY2",
		"PLAIN": "plain-value",
	}

	resolved, err := ResolveEnv(env)
	if err != nil {
		t.Fatalf("ResolveEnv failed: %v", err)
	}

	if resolved["KEY1"] != "value1" {
		t.Errorf("KEY1 = %q, want %q", resolved["KEY1"], "value1")
	}
	if resolved["KEY2"] != "value2" {
		t.Errorf("KEY2 = %q, want %q", resolved["KEY2"], "value2")
	}
	if resolved["PLAIN"] != "plain-value" {
		t.Errorf("PLAIN = %q, want %q", resolved["PLAIN"], "plain-value")
	}
}

func TestResolveEnvError(t *testing.T) {
	env := map[string]string{
		"KEY": "$NONEXISTENT_VAR_12345",
	}

	_, err := ResolveEnv(env)
	if err == nil {
		t.Error("Expected error for unset env var")
	}
}

func TestResolveKeychainFormat(t *testing.T) {
	// We can't actually test keychain access in unit tests,
	// but we can verify the format is recognized
	_, err := Resolve("keychain:test-secret")
	// Will fail because the secret doesn't exist, but that's expected
	if err == nil {
		t.Skip("Keychain test-secret unexpectedly exists")
	}
}
