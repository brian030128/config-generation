package crypto

import (
	"encoding/base64"
	"testing"
)

var testKey = []byte("abcdefghijklmnopqrstuvwxyz123456") // 32 bytes

func TestEncryptDecryptRoundTrip(t *testing.T) {
	plaintext := "hello, world!"
	ct, err := Encrypt(testKey, plaintext)
	if err != nil {
		t.Fatalf("Encrypt error: %v", err)
	}
	got, err := Decrypt(testKey, ct)
	if err != nil {
		t.Fatalf("Decrypt error: %v", err)
	}
	if got != plaintext {
		t.Fatalf("expected %q, got %q", plaintext, got)
	}
}

func TestEncryptDifferentNonces(t *testing.T) {
	plaintext := "same plaintext"
	ct1, err := Encrypt(testKey, plaintext)
	if err != nil {
		t.Fatalf("Encrypt error: %v", err)
	}
	ct2, err := Encrypt(testKey, plaintext)
	if err != nil {
		t.Fatalf("Encrypt error: %v", err)
	}
	if ct1 == ct2 {
		t.Fatal("expected different ciphertexts due to random nonce, but they were identical")
	}
}

func TestDecryptWrongKey(t *testing.T) {
	ct, err := Encrypt(testKey, "secret")
	if err != nil {
		t.Fatalf("Encrypt error: %v", err)
	}
	wrongKey := []byte("zyxwvutsrqponmlkjihgfedcba654321")
	_, err = Decrypt(wrongKey, ct)
	if err == nil {
		t.Fatal("expected error decrypting with wrong key, got nil")
	}
}

func TestDecryptInvalidBase64(t *testing.T) {
	_, err := Decrypt(testKey, "!!!not-base64!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64, got nil")
	}
}

func TestDecryptCiphertextTooShort(t *testing.T) {
	// A valid base64 string that decodes to fewer bytes than the nonce size (12 bytes for GCM).
	short := base64.StdEncoding.EncodeToString([]byte("tooshort"))
	_, err := Decrypt(testKey, short)
	if err == nil {
		t.Fatal("expected error for too-short ciphertext, got nil")
	}
}
