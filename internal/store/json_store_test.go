package store

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestJSONStoreSavesAndUpdatesByUID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "accounts.json")
	s := NewJSONStore(path)
	ctx := context.Background()

	if err := s.Save(ctx, Account{UID: "42", Name: "first", Cookie: "DedeUserID=42; SESSDATA=a; bili_jct=b"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(ctx, Account{UID: "42", Name: "second", Cookie: "DedeUserID=42; SESSDATA=c; bili_jct=d"}); err != nil {
		t.Fatal(err)
	}

	accounts, err := s.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 1 {
		t.Fatalf("account count = %d", len(accounts))
	}
	if accounts[0].UID != "42" || accounts[0].Cookie != "DedeUserID=42; SESSDATA=c; bili_jct=d" {
		t.Fatalf("account not updated: %+v", accounts[0])
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var records []map[string]string
	if err := json.Unmarshal(data, &records); err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || len(records[0]) != 1 || records[0]["cookie"] == "" {
		t.Fatalf("store should persist only cookie records: %s", data)
	}
}

func TestJSONStoreReadsManualCookieOnlyAccount(t *testing.T) {
	path := filepath.Join(t.TempDir(), "accounts.json")
	data := []byte(`[
  {
    "cookie": "DedeUserID=42; SESSDATA=sess; bili_jct=csrf"
  }
]`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	accounts, err := NewJSONStore(path).List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 1 {
		t.Fatalf("account count = %d", len(accounts))
	}
	if accounts[0].UID != "42" || accounts[0].Cookie == "" {
		t.Fatalf("manual account not loaded: %+v", accounts[0])
	}
	if !accounts[0].UpdatedAt.IsZero() {
		t.Fatalf("updatedAt = %v, want zero when omitted", accounts[0].UpdatedAt)
	}
}
