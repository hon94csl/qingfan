package credentials

import (
	"strings"
	"testing"
)

func TestMemoryAndKeychainDoNotEchoTheSecret(t *testing.T) {
	mem := &Memory{}
	if err := mem.Put("local", "workspace-token"); err != nil {
		t.Fatal(err)
	}
	got, err := mem.Get("local")
	if err != nil || got != "workspace-token" {
		t.Fatal(got, err)
	}
	const secret = "workspace-token-value"
	var args []string
	store := Keychain(func(name string, got ...string) ([]byte, error) {
		args = append([]string{name}, got...)
		return nil, errFake
	})
	err = store.Put("local", secret)
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "add-generic-password") || !strings.Contains(joined, "-w "+secret) {
		t.Fatalf("command = %s", joined)
	}
}

var errFake = fakeError("security failed")

type fakeError string

func (e fakeError) Error() string { return string(e) }
