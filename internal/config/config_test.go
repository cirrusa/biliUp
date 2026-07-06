package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadUsesDefaultsAndParsesSupportUPs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	data := []byte(`{
  "task": {
    "numberOfCoins": 3,
    "supportUpIds": [123, 456]
  },
  "storage": {
    "accountsFile": "config/accounts.json"
  }
}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Task.Cron != "0 15 * * *" {
		t.Fatalf("cron default = %q", cfg.Task.Cron)
	}
	if !cfg.Task.WatchVideo || !cfg.Task.ShareVideo || !cfg.Task.SelectLike {
		t.Fatalf("task boolean defaults not applied: %+v", cfg.Task)
	}
	if cfg.Task.NumberOfCoins != 3 {
		t.Fatalf("numberOfCoins = %d", cfg.Task.NumberOfCoins)
	}
	if got := cfg.Task.SupportUpIDs; len(got) != 2 || got[0] != 123 || got[1] != 456 {
		t.Fatalf("supportUpIds = %#v", got)
	}
}

func TestLoadAllowsJSONComments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	data := []byte(`{
  // storage backend
  "storage": {
    "accountsFile": "config/accounts.json" // local account file
  },
  "task": {
    "numberOfCoins": 1,
    /*
      Prefer configured UPs before popular videos.
    */
    "supportUpIds": [123]
  }
}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Storage.AccountsFile != "config/accounts.json" {
		t.Fatalf("accountsFile = %q", cfg.Storage.AccountsFile)
	}
	if cfg.Task.NumberOfCoins != 1 || len(cfg.Task.SupportUpIDs) != 1 || cfg.Task.SupportUpIDs[0] != 123 {
		t.Fatalf("task config not loaded: %+v", cfg.Task)
	}
}
