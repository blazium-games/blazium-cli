package guard

import (
	"os"
	"path/filepath"
	"testing"
)

func TestImportFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "acct.maFile")
	body := `{"account_name":"builder","steamid":"76561197960265728","shared_secret":"YWJjZGVmZ2hpamtsbW5vcHFyc3Q=","identity_secret":"YWJjZGVmZ2hpamtsbW5vcHFyc3Q=","device_id":"android:abcd"}`
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	acc, err := ImportFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if acc.AccountName != "builder" || acc.SharedSecret == "" {
		t.Fatalf("%+v", acc)
	}
	secret, err := SharedSecretFromFlags("", p)
	if err != nil {
		t.Fatal(err)
	}
	if secret != acc.SharedSecret {
		t.Fatalf("%s vs %s", secret, acc.SharedSecret)
	}
}
