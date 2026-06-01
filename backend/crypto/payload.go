package crypto

import (
	"encoding/json"
	"fmt"
)

const MaskedValue = "***"

// EncryptPayload encrypts string values for the given keys in a flat JSON payload.
func EncryptPayload(key []byte, payload json.RawMessage, secretKeys []string) (json.RawMessage, error) {
	if len(secretKeys) == 0 {
		return payload, nil
	}
	var m map[string]any
	if err := json.Unmarshal(payload, &m); err != nil {
		return nil, err
	}
	secret := toSet(secretKeys)
	for k, v := range m {
		if !secret[k] {
			continue
		}
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("secret key %q must have a string value", k)
		}
		enc, err := Encrypt(key, s)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt key %q: %w", k, err)
		}
		m[k] = enc
	}
	return json.Marshal(m)
}

// DecryptPayload decrypts string values for the given keys in a flat JSON payload.
func DecryptPayload(key []byte, payload json.RawMessage, secretKeys []string) (json.RawMessage, error) {
	if len(secretKeys) == 0 {
		return payload, nil
	}
	var m map[string]any
	if err := json.Unmarshal(payload, &m); err != nil {
		return nil, err
	}
	secret := toSet(secretKeys)
	for k, v := range m {
		if !secret[k] {
			continue
		}
		s, ok := v.(string)
		if !ok {
			continue
		}
		dec, err := Decrypt(key, s)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt key %q: %w", k, err)
		}
		m[k] = dec
	}
	return json.Marshal(m)
}

// MaskPayload replaces secret key values with MaskedValue ("***").
func MaskPayload(payload json.RawMessage, secretKeys []string) (json.RawMessage, error) {
	if len(secretKeys) == 0 {
		return payload, nil
	}
	var m map[string]any
	if err := json.Unmarshal(payload, &m); err != nil {
		return nil, err
	}
	secret := toSet(secretKeys)
	for k := range m {
		if secret[k] {
			m[k] = MaskedValue
		}
	}
	return json.Marshal(m)
}

// ResolveSecretKeys computes the final secret_keys for a new version.
// prev is inherited from the previous version; added overrides; unmarked removes entries.
func ResolveSecretKeys(prev, added, unmarked []string) []string {
	result := toSet(prev)
	for _, k := range added {
		result[k] = true
	}
	for _, k := range unmarked {
		delete(result, k)
	}
	out := make([]string, 0, len(result))
	for k := range result {
		out = append(out, k)
	}
	return out
}

func toSet(keys []string) map[string]bool {
	m := make(map[string]bool, len(keys))
	for _, k := range keys {
		m[k] = true
	}
	return m
}
