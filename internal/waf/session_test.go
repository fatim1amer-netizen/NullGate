package waf

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestHashPasswordUsesBcryptAndVerifies(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !strings.HasPrefix(hash, "$2") {
		t.Fatalf("password hash prefix = %q, want bcrypt encoding", hash)
	}
	if !VerifyPassword(hash, "correct horse battery staple") {
		t.Fatal("bcrypt hash did not verify the original password")
	}
	if VerifyPassword(hash, "incorrect password") {
		t.Fatal("bcrypt hash accepted an incorrect password")
	}
}

func TestVerifyPasswordSupportsLegacySHA256AndRejectsPlaintext(t *testing.T) {
	sum := sha256.Sum256([]byte("secret"))
	if !VerifyPassword("sha256:"+hex.EncodeToString(sum[:]), "secret") {
		t.Fatal("legacy SHA-256 hash was not accepted")
	}
	if VerifyPassword("secret", "secret") {
		t.Fatal("plaintext password configuration must not be accepted")
	}
}

func TestHashPasswordRejectsBcryptOverlength(t *testing.T) {
	if _, err := HashPassword(strings.Repeat("x", 73)); err == nil {
		t.Fatal("bcrypt must reject passwords longer than its supported input size")
	}
}
