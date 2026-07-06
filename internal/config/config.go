package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Task     TaskConfig
	Storage  StorageConfig
	Security SecurityConfig
	Logging  LoggingConfig
}

type TaskConfig struct {
	Enabled                bool
	Cron                   string
	WatchVideo             bool
	ShareVideo             bool
	NumberOfCoins          int
	ProtectedCoins         int
	SaveCoinsWhenLv6       bool
	SelectLike             bool
	SupportUpIDs           []int64
	RequestIntervalSeconds int
	TimeoutSeconds         int
	RequestInterval        time.Duration
	Timeout                time.Duration
}

type StorageConfig struct {
	AccountsFile string
}

type SecurityConfig struct {
	UserAgent string
}

type LoggingConfig struct {
	RetentionDays int
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
			AccountsFile: "config/accounts.json",
		},
		Security: SecurityConfig{
			UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36",
		},
		Logging: LoggingConfig{
			RetentionDays: 90,
		},
	}
}

func Load() (Config, error) {
	cfg := Default()
	envFile, err := loadDotEnv(".env")
	if err != nil {
		return Config{}, err
	}

	applyEnv(&cfg, envFile)
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

func applyEnv(cfg *Config, envFile map[string]string) {
	if v, ok := envValue("BILI_UP_TASK_ENABLED", envFile); ok {
		cfg.Task.Enabled = parseBool(v, cfg.Task.Enabled)
	}
	if v, ok := envValue("BILI_UP_TASK_CRON", envFile); ok {
		cfg.Task.Cron = v
	}
	if v, ok := envValue("BILI_UP_WATCH_VIDEO", envFile); ok {
		cfg.Task.WatchVideo = parseBool(v, cfg.Task.WatchVideo)
	}
	if v, ok := envValue("BILI_UP_SHARE_VIDEO", envFile); ok {
		cfg.Task.ShareVideo = parseBool(v, cfg.Task.ShareVideo)
	}
	if v, ok := envValue("BILI_UP_NUMBER_OF_COINS", envFile); ok {
		cfg.Task.NumberOfCoins = parseInt(v, cfg.Task.NumberOfCoins)
	}
	if v, ok := envValue("BILI_UP_PROTECTED_COINS", envFile); ok {
		cfg.Task.ProtectedCoins = parseInt(v, cfg.Task.ProtectedCoins)
	}
	if v, ok := envValue("BILI_UP_SAVE_COINS_WHEN_LV6", envFile); ok {
		cfg.Task.SaveCoinsWhenLv6 = parseBool(v, cfg.Task.SaveCoinsWhenLv6)
	}
	if v, ok := envValue("BILI_UP_SELECT_LIKE", envFile); ok {
		cfg.Task.SelectLike = parseBool(v, cfg.Task.SelectLike)
	}
	if v, ok := envValue("BILI_UP_SUPPORT_UP_IDS", envFile); ok {
		cfg.Task.SupportUpIDs = parseInt64List(v)
	}
	if v, ok := envValue("BILI_UP_REQUEST_INTERVAL_SECONDS", envFile); ok {
		cfg.Task.RequestIntervalSeconds = parseInt(v, cfg.Task.RequestIntervalSeconds)
	}
	if v, ok := envValue("BILI_UP_TIMEOUT_SECONDS", envFile); ok {
		cfg.Task.TimeoutSeconds = parseInt(v, cfg.Task.TimeoutSeconds)
	}
	if v, ok := envValue("BILI_UP_ACCOUNTS_FILE", envFile); ok {
		cfg.Storage.AccountsFile = v
	}
	if v, ok := envValue("BILITOOL_ACCOUNTS_FILE", envFile); ok {
		cfg.Storage.AccountsFile = v
	}
	if v, ok := envValue("BILI_UP_USER_AGENT", envFile); ok {
		cfg.Security.UserAgent = v
	}
	if v, ok := envValue("BILI_UP_LOG_RETENTION_DAYS", envFile); ok {
		cfg.Logging.RetentionDays = parseInt(v, cfg.Logging.RetentionDays)
	}
}

func envValue(key string, envFile map[string]string) (string, bool) {
	if value, ok := os.LookupEnv(key); ok {
		return value, true
	}
	value, ok := envFile[key]
	return value, ok
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

func loadDotEnv(path string) (map[string]string, error) {
	values := map[string]string{}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return values, nil
		}
		return nil, err
	}
	for lineNo, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("%s:%d: expected KEY=value", path, lineNo+1)
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("%s:%d: empty key", path, lineNo+1)
		}
		values[key] = parseDotEnvValue(strings.TrimSpace(value))
	}
	return values, nil
}

func parseDotEnvValue(value string) string {
	if len(value) >= 2 {
		quote := value[0]
		if (quote == '"' || quote == '\'') && value[len(value)-1] == quote {
			return value[1 : len(value)-1]
		}
	}
	if i := strings.Index(value, " #"); i >= 0 {
		value = strings.TrimSpace(value[:i])
	}
	return value
}

func parseBool(value string, fallback bool) bool {
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return parsed
}

func parseInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return parsed
}

func parseInt64List(value string) []int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]int64, 0, len(parts))
	for _, part := range parts {
		parsed, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err != nil {
			continue
		}
		out = append(out, parsed)
	}
	return out
}
