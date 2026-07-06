package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type Config struct {
	Task     TaskConfig     `json:"task"`
	Storage  StorageConfig  `json:"storage"`
	Security SecurityConfig `json:"security"`
}

type TaskConfig struct {
	Enabled                bool          `json:"enabled"`
	Cron                   string        `json:"cron"`
	WatchVideo             bool          `json:"watchVideo"`
	ShareVideo             bool          `json:"shareVideo"`
	NumberOfCoins          int           `json:"numberOfCoins"`
	ProtectedCoins         int           `json:"protectedCoins"`
	SaveCoinsWhenLv6       bool          `json:"saveCoinsWhenLv6"`
	SelectLike             bool          `json:"selectLike"`
	SupportUpIDs           []int64       `json:"supportUpIds"`
	RequestIntervalSeconds int           `json:"requestIntervalSeconds"`
	TimeoutSeconds         int           `json:"timeoutSeconds"`
	RequestInterval        time.Duration `json:"-"`
	Timeout                time.Duration `json:"-"`
}

type StorageConfig struct {
	AccountsFile string `json:"accountsFile"`
}

type SecurityConfig struct {
	UserAgent string `json:"userAgent"`
}

func Default() Config {
	return Config{
		Task: TaskConfig{
			Enabled:                true,
			Cron:                   "0 15 * * *",
			WatchVideo:             true,
			ShareVideo:             true,
			NumberOfCoins:          5,
			ProtectedCoins:         0,
			SelectLike:             true,
			RequestIntervalSeconds: 3,
			TimeoutSeconds:         30,
			RequestInterval:        3 * time.Second,
			Timeout:                30 * time.Second,
		},
		Storage: StorageConfig{
			AccountsFile: "/app/config/accounts.json",
		},
		Security: SecurityConfig{
			UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36",
		},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return Config{}, err
		}
		if err := json.Unmarshal(stripJSONComments(data), &cfg); err != nil {
			return Config{}, err
		}
	}

	applyEnv(&cfg)
	if cfg.Task.RequestIntervalSeconds <= 0 {
		cfg.Task.RequestIntervalSeconds = Default().Task.RequestIntervalSeconds
	}
	if cfg.Task.TimeoutSeconds <= 0 {
		cfg.Task.TimeoutSeconds = Default().Task.TimeoutSeconds
	}
	cfg.Task.RequestInterval = time.Duration(cfg.Task.RequestIntervalSeconds) * time.Second
	cfg.Task.Timeout = time.Duration(cfg.Task.TimeoutSeconds) * time.Second

	if err := validate(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("BILITOOL_ACCOUNTS_FILE"); v != "" {
		cfg.Storage.AccountsFile = v
	}
}

func validate(cfg Config) error {
	if cfg.Task.NumberOfCoins < 0 || cfg.Task.NumberOfCoins > 5 {
		return fmt.Errorf("numberOfCoins must be in [0,5], got %d", cfg.Task.NumberOfCoins)
	}
	if cfg.Task.ProtectedCoins < 0 {
		return fmt.Errorf("protectedCoins must be >= 0, got %d", cfg.Task.ProtectedCoins)
	}
	return nil
}

func stripJSONComments(data []byte) []byte {
	out := make([]byte, 0, len(data))
	inString := false
	escaped := false
	for i := 0; i < len(data); i++ {
		ch := data[i]
		if inString {
			out = append(out, ch)
			if escaped {
				escaped = false
				continue
			}
			switch ch {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}

		if ch == '"' {
			inString = true
			out = append(out, ch)
			continue
		}

		if ch == '/' && i+1 < len(data) {
			next := data[i+1]
			if next == '/' {
				i += 2
				for i < len(data) && data[i] != '\n' && data[i] != '\r' {
					i++
				}
				if i < len(data) {
					out = append(out, data[i])
				}
				continue
			}
			if next == '*' {
				i += 2
				for i+1 < len(data) && !(data[i] == '*' && data[i+1] == '/') {
					if data[i] == '\n' || data[i] == '\r' {
						out = append(out, data[i])
					}
					i++
				}
				i++
				continue
			}
		}
		out = append(out, ch)
	}
	return out
}
