package store

import (
	"context"
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
	if accounts[0].Name != "second" || accounts[0].Cookie != "DedeUserID=42; SESSDATA=c; bili_jct=d" {
		t.Fatalf("account not updated: %+v", accounts[0])
	}
}
