package login

import (
	"context"
	"testing"

	"bili-up/internal/cookie"
	"bili-up/internal/store"
)

func TestServiceLoginSavesCompletedCookieByUID(t *testing.T) {
	bili := &fakeBili{
		generated: QRCode{URL: "https://example.test/qr", Key: "key"},
		cookie:    mustCookie(t, "DedeUserID=42; SESSDATA=sess; bili_jct=csrf"),
	}
	st := &memoryStore{}
	svc := Service{Bili: bili, Store: st, PollLimit: 1}

	account, err := svc.Login(context.Background(), func(string) {})
	if err != nil {
		t.Fatal(err)
	}

	if account.UID != "42" || len(st.accounts) != 1 {
		t.Fatalf("account not saved: account=%+v saved=%+v", account, st.accounts)
	}
	if st.accounts[0].Cookie == "" {
		t.Fatal("saved cookie is empty")
	}
}

func mustCookie(t *testing.T, raw string) *cookie.Cookie {
	t.Helper()
	ck, err := cookie.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return ck
}

type fakeBili struct {
	generated QRCode
	cookie    *cookie.Cookie
}

func (f *fakeBili) GenerateQRCode(ctx context.Context) (QRCode, error) {
	return f.generated, nil
}

func (f *fakeBili) PollQRCode(ctx context.Context, key string) (*cookie.Cookie, bool, error) {
	return f.cookie, true, nil
}

func (f *fakeBili) SetCookie(ctx context.Context, ck *cookie.Cookie) error {
	ck.Items["buvid3"] = "buvid"
	return nil
}

type memoryStore struct {
	accounts []store.Account
}

func (m *memoryStore) List(context.Context) ([]store.Account, error) {
	return m.accounts, nil
}

func (m *memoryStore) Save(_ context.Context, account store.Account) error {
	m.accounts = append(m.accounts, account)
	return nil
}
