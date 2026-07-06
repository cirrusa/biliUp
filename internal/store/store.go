package store

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"bili-up/internal/cookie"
)

type Account struct {
	UID       string    `json:"uid"`
	Name      string    `json:"name,omitempty"`
	Cookie    string    `json:"cookie"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Store interface {
	List(context.Context) ([]Account, error)
	Save(context.Context, Account) error
}

type JSONStore struct {
	path string
}

func NewJSONStore(path string) *JSONStore {
	return &JSONStore{path: path}
}

func (s *JSONStore) List(ctx context.Context) ([]Account, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return []Account{}, nil
	}
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return []Account{}, nil
	}

	var accounts []Account
	if err := json.Unmarshal(data, &accounts); err != nil {
		return nil, err
	}
	for i := range accounts {
		if accounts[i].UID == "" {
			if ck, err := cookie.Parse(accounts[i].Cookie); err == nil {
				accounts[i].UID = ck.UserID()
			}
		}
	}
	return accounts, nil
}

func (s *JSONStore) Save(ctx context.Context, account Account) error {
	accounts, err := s.List(ctx)
	if err != nil {
		return err
	}
	if account.UpdatedAt.IsZero() {
		account.UpdatedAt = time.Now()
	}

	updated := false
	for i := range accounts {
		if accounts[i].UID == account.UID {
			accounts[i] = account
			updated = true
			break
		}
	}
	if !updated {
		accounts = append(accounts, account)
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	records := make([]accountRecord, 0, len(accounts))
	for _, account := range accounts {
		records = append(records, accountRecord{Cookie: account.Cookie})
	}
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

type accountRecord struct {
	Cookie string `json:"cookie"`
}
