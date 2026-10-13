package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// TestLoadHashesPlaintextPassword: a password typed into config.yaml is
// replaced by its bcrypt hash on the first load and the file rewritten, so
// the plain text neither stays on disk nor ends up in backups. The hashed
// file loads again without change, and both forms verify.
func TestLoadHashesPlaintextPassword(t *testing.T) {
	BcryptCost = bcrypt.MinCost
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("auth:\n  username: stefan\n  password: geheim123\n"), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := Load(path); err != nil {
		t.Fatalf("load: %v", err)
	}

	auth := GetAuth()
	if auth.Password != "" || !strings.HasPrefix(auth.PasswordHash, "$2") {
		t.Fatalf("auth after load = %+v, want hashed password and no plain text", auth)
	}
	if !auth.Enabled() || auth.User() != "stefan" {
		t.Errorf("auth = %+v, want enabled for stefan", auth)
	}
	if !auth.Verify("geheim123") || auth.Verify("falsch") {
		t.Error("hash does not verify the original password only")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if strings.Contains(string(data), "geheim123") {
		t.Errorf("plain-text password still on disk:\n%s", data)
	}
	if !strings.Contains(string(data), "password_hash:") {
		t.Errorf("hash not written:\n%s", data)
	}

	hashBefore := auth.PasswordHash
	if _, err := Load(path); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if GetAuth().PasswordHash != hashBefore {
		t.Error("reloading a hashed config changed the hash")
	}

	// Disabling clears everything.
	if err := UpdateAuth(AuthConfig{}); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if GetAuth().Enabled() {
		t.Error("auth still enabled after UpdateAuth with no password")
	}
}
