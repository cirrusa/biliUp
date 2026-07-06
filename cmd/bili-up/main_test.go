package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPrintLoginQRCode(t *testing.T) {
	const url = "https://example.test/login?qrcode_key=test"

	var out bytes.Buffer
	printLoginQRCode(url, &out)

	got := out.String()
	if !strings.Contains(got, "Scan this QR code with Bilibili app:") {
		t.Fatalf("output missing scan prompt: %q", got)
	}
	if !strings.Contains(got, url) {
		t.Fatalf("output missing fallback url: %q", got)
	}
	if strings.Count(got, "\n") < 4 {
		t.Fatalf("output too short to include qr code: %q", got)
	}
}

func TestDailyLoggerWritesToFileAndStdout(t *testing.T) {
	dir := t.TempDir()
	location := time.FixedZone("Asia/Shanghai", 8*60*60)
	now := fixedNow(location, "2026-07-06")
	var stdout bytes.Buffer

	logger, closer, err := newDailyLogger(dir, 90, &stdout, location, now)
	if err != nil {
		t.Fatal(err)
	}
	logger.Print("hello daily log")
	closeLogger(closer)

	fileData, err := os.ReadFile(filepath.Join(dir, "bili-up-2026-07-06.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(fileData), "hello daily log") {
		t.Fatalf("file log missing message: %q", fileData)
	}
	if !strings.Contains(stdout.String(), "hello daily log") {
		t.Fatalf("stdout log missing message: %q", stdout.String())
	}
}

func TestDailyLoggerCreatesLogDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "logs")
	location := time.FixedZone("Asia/Shanghai", 8*60*60)

	logger, closer, err := newDailyLogger(dir, 90, ioDiscard{}, location, fixedNow(location, "2026-07-06"))
	if err != nil {
		t.Fatal(err)
	}
	logger.Print("created")
	closeLogger(closer)

	if _, err := os.Stat(filepath.Join(dir, "bili-up-2026-07-06.log")); err != nil {
		t.Fatal(err)
	}
}

func TestDailyLoggerRotatesByDate(t *testing.T) {
	dir := t.TempDir()
	location := time.FixedZone("Asia/Shanghai", 8*60*60)
	now := time.Date(2026, 7, 6, 23, 59, 0, 0, location)

	logger, closer, err := newDailyLogger(dir, 90, ioDiscard{}, location, func() time.Time {
		return now
	})
	if err != nil {
		t.Fatal(err)
	}
	logger.Print("day one")
	now = time.Date(2026, 7, 7, 0, 1, 0, 0, location)
	logger.Print("day two")
	closeLogger(closer)

	for _, name := range []string{"bili-up-2026-07-06.log", "bili-up-2026-07-07.log"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("%s missing: %v", name, err)
		}
	}
}

func TestDailyLoggerCleansExpiredFiles(t *testing.T) {
	dir := t.TempDir()
	location := time.FixedZone("Asia/Shanghai", 8*60*60)
	files := []string{
		"bili-up-2026-04-07.log",
		"bili-up-2026-04-08.log",
		"custom.log",
	}
	for _, name := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("old"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	_, closer, err := newDailyLogger(dir, 90, ioDiscard{}, location, fixedNow(location, "2026-07-06"))
	if err != nil {
		t.Fatal(err)
	}
	closeLogger(closer)

	if _, err := os.Stat(filepath.Join(dir, "bili-up-2026-04-07.log")); !os.IsNotExist(err) {
		t.Fatalf("expired log was not removed: %v", err)
	}
	for _, name := range []string{"bili-up-2026-04-08.log", "custom.log", "bili-up-2026-07-06.log"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("%s should remain: %v", name, err)
		}
	}
}

func TestRunLogsReturnedError(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatal(err)
		}
	})
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("not-valid\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := run([]string{"accounts"}); err == nil {
		t.Fatal("expected .env parse error")
	}

	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	today := time.Now().In(location).Format(logDateLayout)
	data, err := os.ReadFile(filepath.Join(dir, "logs", "bili-up-"+today+".log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "error:") {
		t.Fatalf("log missing returned error: %q", data)
	}
}

func TestRunUsesConfiguredLogRetention(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatal(err)
		}
	})

	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	today := time.Now().In(location)
	oldName := logFilePrefix + today.AddDate(0, 0, -1).Format(logDateLayout) + logFileSuffix
	if err := os.MkdirAll("logs", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("logs", oldName), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(`BILI_UP_ACCOUNTS_FILE=accounts.json
BILI_UP_LOG_RETENTION_DAYS=1
`), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := run([]string{"accounts"}); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join("logs", oldName)); !os.IsNotExist(err) {
		t.Fatalf("old log was not removed by configured retention: %v", err)
	}
	currentName := logFilePrefix + today.Format(logDateLayout) + logFileSuffix
	if _, err := os.Stat(filepath.Join("logs", currentName)); err != nil {
		t.Fatalf("current log missing: %v", err)
	}
}

func fixedNow(location *time.Location, date string) func() time.Time {
	return func() time.Time {
		t, err := time.ParseInLocation(logDateLayout, date, location)
		if err != nil {
			panic(err)
		}
		return t
	}
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) {
	return len(p), nil
}
