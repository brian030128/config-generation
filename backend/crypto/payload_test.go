package crypto

import (
	"encoding/json"
	"testing"
)

func mustMarshal(t *testing.T, v map[string]any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return b
}

func mustUnmarshal(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	return m
}

func TestEncryptPayloadEncryptsOnlySecretKeys(t *testing.T) {
	payload := mustMarshal(t, map[string]any{
		"username": "alice",
		"password": "s3cret",
	})
	result, err := EncryptPayload(testKey, payload, []string{"password"})
	if err != nil {
		t.Fatalf("EncryptPayload error: %v", err)
	}
	m := mustUnmarshal(t, result)
	if m["username"] != "alice" {
		t.Errorf("expected username to be unchanged, got %v", m["username"])
	}
	if m["password"] == "s3cret" {
		t.Error("expected password to be encrypted, but it was unchanged")
	}
}

func TestEncryptPayloadErrorOnNonStringSecretKey(t *testing.T) {
	payload := mustMarshal(t, map[string]any{
		"count": 42,
	})
	_, err := EncryptPayload(testKey, payload, []string{"count"})
	if err == nil {
		t.Fatal("expected error for non-string secret key value, got nil")
	}
}

func TestDecryptPayloadRoundTrip(t *testing.T) {
	original := map[string]any{
		"username": "alice",
		"password": "s3cret",
	}
	payload := mustMarshal(t, original)
	encrypted, err := EncryptPayload(testKey, payload, []string{"password"})
	if err != nil {
		t.Fatalf("EncryptPayload error: %v", err)
	}
	decrypted, err := DecryptPayload(testKey, encrypted, []string{"password"})
	if err != nil {
		t.Fatalf("DecryptPayload error: %v", err)
	}
	m := mustUnmarshal(t, decrypted)
	if m["password"] != "s3cret" {
		t.Errorf("expected password %q after decrypt, got %v", "s3cret", m["password"])
	}
	if m["username"] != "alice" {
		t.Errorf("expected username %q, got %v", "alice", m["username"])
	}
}

func TestDecryptPayloadNoOpWhenNoSecretKeys(t *testing.T) {
	payload := mustMarshal(t, map[string]any{"foo": "bar"})
	result, err := DecryptPayload(testKey, payload, []string{})
	if err != nil {
		t.Fatalf("DecryptPayload error: %v", err)
	}
	m := mustUnmarshal(t, result)
	if m["foo"] != "bar" {
		t.Errorf("expected foo to be unchanged, got %v", m["foo"])
	}
}

func TestMaskPayloadReplacesSecretKeys(t *testing.T) {
	payload := mustMarshal(t, map[string]any{
		"username": "alice",
		"password": "s3cret",
	})
	result, err := MaskPayload(payload, []string{"password"})
	if err != nil {
		t.Fatalf("MaskPayload error: %v", err)
	}
	m := mustUnmarshal(t, result)
	if m["password"] != MaskedValue {
		t.Errorf("expected password to be %q, got %v", MaskedValue, m["password"])
	}
	if m["username"] != "alice" {
		t.Errorf("expected username to be unchanged, got %v", m["username"])
	}
}

func TestMaskPayloadNoOpWhenNoSecretKeys(t *testing.T) {
	payload := mustMarshal(t, map[string]any{"foo": "bar"})
	result, err := MaskPayload(payload, []string{})
	if err != nil {
		t.Fatalf("MaskPayload error: %v", err)
	}
	m := mustUnmarshal(t, result)
	if m["foo"] != "bar" {
		t.Errorf("expected foo to be unchanged, got %v", m["foo"])
	}
}

func TestResolveSecretKeysMergesPrevAddedUnmarked(t *testing.T) {
	prev := []string{"password"}
	added := []string{"api_key"}
	unmarked := []string{"password"}

	result := ResolveSecretKeys(prev, added, unmarked)

	has := func(slice []string, key string) bool {
		for _, k := range slice {
			if k == key {
				return true
			}
		}
		return false
	}

	if has(result, "password") {
		t.Error("expected password to be removed from secret keys")
	}
	if !has(result, "api_key") {
		t.Error("expected api_key to be in secret keys")
	}
}

func TestResolveSecretKeysInheritsFromPrev(t *testing.T) {
	result := ResolveSecretKeys([]string{"password"}, nil, nil)
	found := false
	for _, k := range result {
		if k == "password" {
			found = true
		}
	}
	if !found {
		t.Error("expected password to be inherited from prev")
	}
}
