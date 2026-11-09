package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateRequiresSafeAccount(t *testing.T) {
	dir := t.TempDir()
	err := Validate(Config{
		VaultPath: dir,
		Account:   "../i568439",
		Role:      RoleSpooler,
		SpoolDir:  DefaultSpoolDir,
		Roots: []Root{
			{Path: dir, Bucket: "Hackastak"},
			{Path: dir, Bucket: "SAP Business Network & ANT"},
		},
	})
	if err == nil {
		t.Fatal("Validate() error = nil")
	}
	if !hasField(err, "account") {
		t.Errorf("error = %q, want account", err)
	}
	if hasField(err, "roots") {
		t.Errorf("error = %q, two buckets are allowed", err)
	}
}

func TestValidateRejectsSpoolDirOutsideVault(t *testing.T) {
	dir := t.TempDir()
	err := Validate(Config{
		VaultPath: dir,
		Account:   "hackastak",
		Role:      RoleWriter,
		SpoolDir:  t.TempDir(),
		Roots:     []Root{{Path: dir, Bucket: "Hackastak"}},
	})
	if err == nil {
		t.Fatal("Validate() error = nil")
	}
	if !hasField(err, "spool_dir") {
		t.Errorf("error = %q, want spool_dir", err)
	}
}

func TestLoadResolvesDefaultSpoolDirInsideVault(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, `
vault_path = "`+dir+`"
account = "i568439"
role = "spooler"

[[roots]]
path = "`+dir+`"
bucket = "SAP Business Network & ANT"
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := filepath.Join(dir, "_Weekly", ".hackatrak")
	if cfg.SpoolDir != want {
		t.Errorf("SpoolDir = %q, want %q", cfg.SpoolDir, want)
	}
	if cfg.Role != RoleSpooler {
		t.Errorf("Role = %q", cfg.Role)
	}
	buckets, err := Buckets(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(buckets) != 1 || buckets[0] != "SAP Business Network & ANT" {
		t.Errorf("Buckets() = %#v", buckets)
	}
	if _, err := os.Stat(cfg.SpoolDir); !os.IsNotExist(err) {
		t.Fatalf("default spool dir stat = %v, want not exist", err)
	}
}

func TestBucketsTrimsAndKeepsEachRoot(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		VaultPath: dir,
		Account:   "hackastak",
		Role:      RoleWriter,
		Roots: []Root{
			{Path: dir, Bucket: " Hackastak "},
			{Path: dir, Bucket: "SAP Business Network & ANT"},
		},
	}
	got, err := Buckets(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "Hackastak" || got[1] != "SAP Business Network & ANT" {
		t.Errorf("Buckets() = %#v", got)
	}
}

func TestCheckAccount(t *testing.T) {
	if err := CheckAccount("i568439"); err != nil {
		t.Fatal(err)
	}
	if err := CheckAccount("hackastak"); err != nil {
		t.Fatal(err)
	}
	for _, account := range []string{"", " ", "a/b", `a\b`, "..", "a..b", ".hidden", "-x"} {
		if err := CheckAccount(account); err == nil {
			t.Errorf("CheckAccount(%q) error = nil", account)
		}
	}
	if !strings.Contains(CheckAccount("").Error(), "account") {
		t.Errorf("empty account error = %q", CheckAccount(""))
	}
}
