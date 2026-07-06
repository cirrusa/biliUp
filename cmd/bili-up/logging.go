package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	logFilePrefix = "bili-up-"
	logFileSuffix = ".log"
	logDateLayout = "2006-01-02"
)

type dailyLogWriter struct {
	mu            sync.Mutex
	dir           string
	retentionDays int
	location      *time.Location
	now           func() time.Time
	currentDate   string
	file          *os.File
}

func newRuntimeLogger(retentionDays int, stdout io.Writer) (*log.Logger, io.Closer, error) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return nil, nil, err
	}
	return newDailyLogger("logs", retentionDays, stdout, location, time.Now)
}

func newDailyLogger(logDir string, retentionDays int, stdout io.Writer, location *time.Location, now func() time.Time) (*log.Logger, io.Closer, error) {
	writer, err := newDailyLogWriter(logDir, retentionDays, location, now)
	if err != nil {
		return nil, nil, err
	}
	return log.New(io.MultiWriter(stdout, writer), "", log.LstdFlags), writer, nil
}

func newDailyLogWriter(dir string, retentionDays int, location *time.Location, now func() time.Time) (*dailyLogWriter, error) {
	w := &dailyLogWriter{
		dir:           dir,
		retentionDays: retentionDays,
		location:      location,
		now:           now,
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	if err := w.rotateLocked(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *dailyLogWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.rotateLocked(); err != nil {
		return 0, err
	}
	return w.file.Write(p)
}

func (w *dailyLogWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}

func (w *dailyLogWriter) rotateLocked() error {
	today := w.now().In(w.location).Format(logDateLayout)
	if w.file != nil && w.currentDate == today {
		return nil
	}
	if w.file != nil {
		if err := w.file.Close(); err != nil {
			return err
		}
		w.file = nil
	}
	file, err := os.OpenFile(filepath.Join(w.dir, logFilePrefix+today+logFileSuffix), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	w.file = file
	w.currentDate = today
	return w.cleanupLocked()
}

func (w *dailyLogWriter) cleanupLocked() error {
	if w.retentionDays <= 0 {
		return nil
	}
	cutoff := startOfDay(w.now().In(w.location), w.location).AddDate(0, 0, -w.retentionDays+1)
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		date, ok := logFileDate(entry.Name(), w.location)
		if !ok || !date.Before(cutoff) {
			continue
		}
		if err := os.Remove(filepath.Join(w.dir, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func logFileDate(name string, location *time.Location) (time.Time, bool) {
	if !strings.HasPrefix(name, logFilePrefix) || !strings.HasSuffix(name, logFileSuffix) {
		return time.Time{}, false
	}
	datePart := strings.TrimSuffix(strings.TrimPrefix(name, logFilePrefix), logFileSuffix)
	if len(datePart) != len(logDateLayout) {
		return time.Time{}, false
	}
	date, err := time.ParseInLocation(logDateLayout, datePart, location)
	if err != nil {
		return time.Time{}, false
	}
	return date, true
}

func startOfDay(t time.Time, location *time.Location) time.Time {
	year, month, day := t.In(location).Date()
	return time.Date(year, month, day, 0, 0, 0, 0, location)
}

func closeLogger(closer io.Closer) {
	if closer == nil {
		return
	}
	if err := closer.Close(); err != nil {
		fmt.Fprintln(os.Stderr, "log close error:", err)
	}
}
