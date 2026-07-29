package remote

import (
	"strings"
	"testing"
	"time"
)

func TestGenerateInstanceIDFormat(t *testing.T) {
	for i := 0; i < 20; i++ {
		id, err := GenerateInstanceID()
		if err != nil {
			t.Fatal(err)
		}
		if len(id) != 6 {
			t.Fatalf("len=%d id=%q", len(id), id)
		}
		for _, r := range id {
			if !strings.ContainsRune(instanceIDAlphabet, r) {
				t.Fatalf("invalid rune %c in %q", r, id)
			}
		}
		for _, bad := range []rune{'0', 'O', '1', 'I', 'o', 'l'} {
			if strings.ContainsRune(id, bad) {
				t.Fatalf("ambiguous rune %c in %q", bad, id)
			}
		}
	}
}

func TestAllocatePortsSkipsClaimed(t *testing.T) {
	claimed := map[int]bool{6500: true, 6501: true}
	ports, err := AllocatePorts("127.0.0.1", 2, claimed)
	if err != nil {
		t.Fatal(err)
	}
	if ports[0] < 6502 {
		t.Fatalf("expected skip claimed, got %v", ports)
	}
	if ports[0] == ports[1] {
		t.Fatal("duplicate ports")
	}
}

func TestSortNewestFirst(t *testing.T) {
	now := time.Now()
	items := []RemoteInstance{
		{ID: "AAAAAA", StartedAt: now.Add(-time.Hour)},
		{ID: "BBBBBB", StartedAt: now},
	}
	sorted := SortNewestFirst(items)
	if sorted[0].ID != "BBBBBB" {
		t.Fatalf("got %s", sorted[0].ID)
	}
}

func TestGenerateTokenLength(t *testing.T) {
	tok, err := GenerateToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(tok) != 64 {
		t.Fatalf("len=%d", len(tok))
	}
}
