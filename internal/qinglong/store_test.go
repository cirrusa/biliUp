package qinglong

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"bilitool-go/internal/store"
)

func TestStoreUpdatesExistingCookieEnvByUID(t *testing.T) {
	var updated updateEnvRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/open/auth/token":
			_ = json.NewEncoder(w).Encode(tokenResponse{Code: 200, Data: tokenData{TokenType: "Bearer", Token: "abc"}})
		case "/open/envs":
			if r.Method == http.MethodGet {
				_ = json.NewEncoder(w).Encode(envListResponse{Code: 200, Data: []Env{{ID: 7, Name: "Ray_BiliBiliCookies__0", Value: "DedeUserID=42; SESSDATA=old; bili_jct=old"}}})
				return
			}
			if r.Method != http.MethodPut {
				t.Fatalf("unexpected %s /open/envs", r.Method)
			}
			if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
				t.Fatal(err)
			}
			_ = json.NewEncoder(w).Encode(genericResponse{Code: 200})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	s := NewStore(NewClient(server.URL, "id", "secret"))
	err := s.Save(context.Background(), store.Account{UID: "42", Cookie: "DedeUserID=42; SESSDATA=new; bili_jct=new"})
	if err != nil {
		t.Fatal(err)
	}

	if updated.ID != 7 || updated.Name != "Ray_BiliBiliCookies__0" || updated.Value != "DedeUserID=42; SESSDATA=new; bili_jct=new" {
		t.Fatalf("unexpected update request: %+v", updated)
	}
}

func TestStoreAddsNextCookieEnvWhenUIDDoesNotExist(t *testing.T) {
	var added []addEnvRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/open/auth/token":
			_ = json.NewEncoder(w).Encode(tokenResponse{Code: 200, Data: tokenData{TokenType: "Bearer", Token: "abc"}})
		case "/open/envs":
			if r.Method == http.MethodGet {
				_ = json.NewEncoder(w).Encode(envListResponse{Code: 200, Data: []Env{{ID: 1, Name: "Ray_BiliBiliCookies__0", Value: "DedeUserID=1; SESSDATA=a; bili_jct=b"}}})
				return
			}
			if err := json.NewDecoder(r.Body).Decode(&added); err != nil {
				t.Fatal(err)
			}
			_ = json.NewEncoder(w).Encode(genericResponse{Code: 200})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	s := NewStore(NewClient(server.URL, "id", "secret"))
	err := s.Save(context.Background(), store.Account{UID: "42", Cookie: "DedeUserID=42; SESSDATA=new; bili_jct=new"})
	if err != nil {
		t.Fatal(err)
	}

	if len(added) != 1 || added[0].Name != "Ray_BiliBiliCookies__1" || added[0].Remarks != "bili-42" {
		t.Fatalf("unexpected add request: %+v", added)
	}
}
