package config

import (
	"os"
	"path/filepath"
	"testing"
)

var configEnvKeys = []string{
	"BILI_UP_TASK_ENABLED",
	"BILI_UP_TASK_CRON",
	"BILI_UP_WATCH_VIDEO",
	"BILI_UP_SHARE_VIDEO",
	"BILI_UP_NUMBER_OF_COINS",
	"BILI_UP_PROTECTED_COINS",
	"BILI_UP_SAVE_COINS_WHEN_LV6",
	"BILI_UP_SELECT_LIKE",
	"BILI_UP_SUPPORT_UP_IDS",
	"BILI_UP_REQUEST_INTERVAL_SECONDS",
	"BILI_UP_TIMEOUT_SECONDS",
	"BILI_UP_ACCOUNTS_FILE",
	"BILITOOL_ACCOUNTS_FILE",
	"BILI_UP_USER_AGENT",
	"BILI_UP_LOG_RETENTION_DAYS",
}

func TestLoadUsesDefaults(t *testing.T) {
	withCleanConfigEnv(t)
	withWorkingDir(t, t.TempDir())

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Task.Cron != "0 15 * * *" {
		t.Fatalf("cron default = %q", cfg.Task.Cron)
	}
	if !cfg.Task.WatchVideo || !cfg.Task.ShareVideo || !cfg.Task.SaveCoinsWhenLv6 || !cfg.Task.SelectLike {
		t.Fatalf("task boolean defaults not applied: %+v", cfg.Task)
	}
	if cfg.Storage.AccountsFile != "config/accounts.json" {
		t.Fatalf("accountsFile default = %q", cfg.Storage.AccountsFile)
	}
	if cfg.Logging.RetentionDays != 90 {
		t.Fatalf("log retention default = %d", cfg.Logging.RetentionDays)
	}
}

func TestLoadReadsDotEnv(t *testing.T) {
	withCleanConfigEnv(t)
	dir := t.TempDir()
	withWorkingDir(t, dir)
	data := []byte(`BILI_UP_TASK_CRON=30 6 * * *
BILI_UP_WATCH_VIDEO=false
BILI_UP_SHARE_VIDEO=true
BILI_UP_NUMBER_OF_COINS=3
BILI_UP_PROTECTED_COINS=10
BILI_UP_SAVE_COINS_WHEN_LV6=true
BILI_UP_SELECT_LIKE=false
BILI_UP_SUPPORT_UP_IDS=123,456
BILI_UP_REQUEST_INTERVAL_SECONDS=5
BILI_UP_TIMEOUT_SECONDS=60
BILI_UP_ACCOUNTS_FILE=config/custom-accounts.json
BILI_UP_LOG_RETENTION_DAYS=30
`)
	if err := os.WriteFile(filepath.Join(dir, ".env"), data, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Task.Cron != "30 6 * * *" {
		t.Fatalf("cron = %q", cfg.Task.Cron)
	}
	if cfg.Task.WatchVideo || !cfg.Task.ShareVideo || !cfg.Task.SaveCoinsWhenLv6 || cfg.Task.SelectLike {
		t.Fatalf("task booleans not loaded: %+v", cfg.Task)
	}
	if cfg.Task.NumberOfCoins != 3 || cfg.Task.ProtectedCoins != 10 {
		t.Fatalf("coin settings not loaded: %+v", cfg.Task)
	}
	if got := cfg.Task.SupportUpIDs; len(got) != 2 || got[0] != 123 || got[1] != 456 {
		t.Fatalf("support UP IDs = %#v", got)
	}
	if cfg.Task.RequestIntervalSeconds != 5 || cfg.Task.TimeoutSeconds != 60 {
		t.Fatalf("timing settings not loaded: %+v", cfg.Task)
	}
	if cfg.Storage.AccountsFile != "config/custom-accounts.json" {
		t.Fatalf("accountsFile = %q", cfg.Storage.AccountsFile)
	}
	if cfg.Logging.RetentionDays != 30 {
		t.Fatalf("log retention = %d", cfg.Logging.RetentionDays)
	}
}

func TestLoadEnvOverridesDotEnv(t *testing.T) {
	withCleanConfigEnv(t)
	dir := t.TempDir()
	withWorkingDir(t, dir)
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("BILI_UP_NUMBER_OF_COINS=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BILI_UP_NUMBER_OF_COINS", "4")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Task.NumberOfCoins != 4 {
		t.Fatalf("numberOfCoins = %d", cfg.Task.NumberOfCoins)
	}
}

func TestLoadAllowsDisablingLogCleanup(t *testing.T) {
	withCleanConfigEnv(t)
	dir := t.TempDir()
	withWorkingDir(t, dir)
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("BILI_UP_LOG_RETENTION_DAYS=0\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Logging.RetentionDays != 0 {
		t.Fatalf("log retention = %d", cfg.Logging.RetentionDays)
	}
}

func TestLoadRejectsInvalidDotEnvLine(t *testing.T) {
	withCleanConfigEnv(t)
	dir := t.TempDir()
	withWorkingDir(t, dir)
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("not-valid\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(); err == nil {
		t.Fatal("expected invalid .env error")
	}
}

func withWorkingDir(t *testing.T, dir string) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatal(err)
		}
	})
}

func withCleanConfigEnv(t *testing.T) {
	t.Helper()
	previous := make(map[string]string, len(configEnvKeys))
	exists := make(map[string]bool, len(configEnvKeys))
	for _, key := range configEnvKeys {
		value, ok := os.LookupEnv(key)
		previous[key] = value
		exists[key] = ok
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, key := range configEnvKeys {
			if exists[key] {
				_ = os.Setenv(key, previous[key])
			} else {
				_ = os.Unsetenv(key)
			}
		}
	})
}
