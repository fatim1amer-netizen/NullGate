package main

import (
	"path/filepath"
	"testing"

	"nullgate/internal/waf"
)

func TestShowcaseAdminCredentialIsGeneratedOnceAndPersistsAsHash(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials", "admin.hash")
	password, passwordHash, created, err := loadOrCreateShowcaseAdmin(path)
	if err != nil {
		t.Fatal(err)
	}
	if !created || password == "" || !waf.VerifyPassword(passwordHash, password) {
		t.Fatal("first launch did not create a verifiable one-time credential")
	}
	secondPassword, secondHash, created, err := loadOrCreateShowcaseAdmin(path)
	if err != nil {
		t.Fatal(err)
	}
	if created || secondPassword != "" || secondHash != passwordHash {
		t.Fatal("persisted credential was not reused as a hash")
	}
}
