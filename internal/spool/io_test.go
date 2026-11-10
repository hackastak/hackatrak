package spool

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteReadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "spool-i568439-2026-W01.json")
	file := File{
		Version: Version,
		Account: "i568439",
		Year:    2026,
		Week:    1,
		Days: []Day{{
			Date:   "2026-01-01",
			Bucket: "SAP Business Network & ANT",
			Slug:   "hackatrak",
			Ticket: "NEXTGEN-1",
			Text:   "- [[hackatrak]] (NEXTGEN-1): defined the spool",
		}},
	}

	wrote, err := Write(path, file)
	if err != nil {
		t.Fatal(err)
	}
	if !wrote {
		t.Fatal("Write() wrote = false, want a new file")
	}
	got, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Account != file.Account || len(got.Days) != 1 || got.Days[0].Slug != "hackatrak" {
		t.Fatalf("Read() = %#v", got)
	}

	again, err := Write(path, file)
	if err != nil {
		t.Fatal(err)
	}
	if again {
		t.Fatal("second Write() rewrote equal content")
	}
}

func TestReadRejectsMismatchedName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spool-other-2026-W01.json")
	body := []byte(`{"version":1,"account":"i568439","year":2026,"week":1,"days":[]}`)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(path); err == nil {
		t.Fatal("Read() error = nil, want a filename mismatch")
	}
}

func TestDiscoverCurrentWeek(t *testing.T) {
	dir := t.TempDir()
	keep := filepath.Join(dir, "spool-i568439-2026-W01.json")
	other := filepath.Join(dir, "spool-i568439-2026-W02.json")
	junk := filepath.Join(dir, "notes.md")
	for _, path := range []string{keep, other, junk} {
		if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "spool-not-an-account-2026-W01.json"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := Discover(dir, 2026, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != keep {
		t.Fatalf("Discover() = %#v, want only the W01 file", got)
	}

	missing, err := Discover(filepath.Join(dir, "absent"), 2026, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 0 {
		t.Fatalf("missing dir = %#v, want empty", missing)
	}
}
