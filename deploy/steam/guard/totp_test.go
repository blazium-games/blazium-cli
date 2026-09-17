package guard

import (
	"encoding/base64"
	"testing"
)

func TestGenerateAuthCodeAt(t *testing.T) {
	secret := base64.StdEncoding.EncodeToString([]byte("12345678901234567890"))
	code, err := GenerateAuthCodeAt(secret, 59)
	if err != nil {
		t.Fatal(err)
	}
	if len(code) != 5 {
		t.Fatalf("len %d %q", len(code), code)
	}
	for _, c := range code {
		if !containsRune(alphabet, c) {
			t.Fatalf("char %c not in alphabet: %s", c, code)
		}
	}
	again, err := GenerateAuthCodeAt(secret, 59)
	if err != nil {
		t.Fatal(err)
	}
	if again != code {
		t.Fatalf("%s vs %s", again, code)
	}
	other, err := GenerateAuthCodeAt(secret, 90)
	if err != nil {
		t.Fatal(err)
	}
	if other == code {
		t.Fatal("expected different window")
	}
}

func TestHexSecret(t *testing.T) {
	hexSecret := "3132333435363738393031323334353637383930" // 12345678901234567890
	a, err := GenerateAuthCodeAt(hexSecret, 59)
	if err != nil {
		t.Fatal(err)
	}
	b64 := base64.StdEncoding.EncodeToString([]byte("12345678901234567890"))
	b, err := GenerateAuthCodeAt(b64, 59)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("%s vs %s", a, b)
	}
}

func TestConfirmationAndDevice(t *testing.T) {
	secret := base64.StdEncoding.EncodeToString([]byte("12345678901234567890"))
	k, err := GenerateConfirmationKey(secret, 1000, "conf")
	if err != nil || k == "" {
		t.Fatalf("%q %v", k, err)
	}
	id := DeviceID("76561197960265728")
	if id[:8] != "android:" || len(id) < 20 {
		t.Fatalf("%s", id)
	}
}

func containsRune(s string, r rune) bool {
	for _, c := range s {
		if c == r {
			return true
		}
	}
	return false
}
