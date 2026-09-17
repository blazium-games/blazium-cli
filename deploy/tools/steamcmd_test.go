package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocateSteamcmdEnv(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "steamcmd.exe")
	if err := os.WriteFile(p, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BLAZIUM_STEAMCMD", p)
	st := LocateSteamcmd()
	if !st.Ready || st.Steamcmd != p || st.Source != envSteamcmd {
		t.Fatalf("%+v", st)
	}
}
