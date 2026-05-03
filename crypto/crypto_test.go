package crypto

import (
	"strings"
	"testing"
)

func TestEncryptedStringValueFailsClosedWithoutCryptoService(t *testing.T) {
	original := globalCryptoService
	defer SetGlobalCryptoService(original)
	SetGlobalCryptoService(nil)

	_, err := EncryptedString("super-secret").Value()
	if err == nil {
		t.Fatal("expected Value to fail when encryption service is not initialized")
	}
}

func TestEncryptedStringValueEncryptsWhenCryptoAvailable(t *testing.T) {
	cs := &CryptoService{dataKey: []byte("0123456789abcdef0123456789abcdef")}
	original := globalCryptoService
	defer SetGlobalCryptoService(original)
	SetGlobalCryptoService(cs)

	value, err := EncryptedString("super-secret").Value()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	encrypted, ok := value.(string)
	if !ok {
		t.Fatalf("expected encrypted value to be string, got %T", value)
	}
	if !strings.HasPrefix(encrypted, storagePrefix) {
		t.Fatalf("expected encrypted storage prefix, got %q", encrypted)
	}
	if strings.Contains(encrypted, "super-secret") {
		t.Fatal("encrypted value should not contain plaintext")
	}
}
